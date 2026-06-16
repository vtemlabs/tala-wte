// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

// Package capture manages packet captures via tshark with tcpdump fallback.
package capture

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const CaptureDir = "/var/lib/tala-wte/captures"

// Layer defines whether to capture at wireless or network layer.
type Layer string

const (
	LayerWireless Layer = "wireless"
	LayerNetwork  Layer = "network"
)

// unsafeBPFPattern matches shell metacharacters that should not appear in BPF filters.
var unsafeBPFPattern = regexp.MustCompile("[;|&`$\\\\\"'{}]")

// ValidateBPFFilter rejects shell metacharacters and control characters, then
// verifies syntax by compiling the filter with tcpdump.
func ValidateBPFFilter(filter string) error {
	if filter == "" {
		return nil
	}
	if unsafeBPFPattern.MatchString(filter) {
		return fmt.Errorf("invalid BPF filter: contains disallowed characters")
	}
	if strings.ContainsAny(filter, "\n\r\x00") {
		return fmt.Errorf("invalid BPF filter: contains control characters")
	}
	if _, err := exec.LookPath("tcpdump"); err == nil {
		if out, err := exec.Command("tcpdump", "-d", "-i", "lo", filter).CombinedOutput(); err != nil {
			return fmt.Errorf("invalid BPF filter syntax: %s", strings.TrimSpace(string(out)))
		}
	}
	return nil
}

// Session represents an active or completed packet capture.
type Session struct {
	ID        string
	Interface string
	Layer     Layer
	Filter    string
	FilePath  string
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	onExit    func(packetCount int)
	finished  chan int
}

var (
	mu       sync.Mutex
	sessions = make(map[string]*Session)
)

// validateIface rejects interface names that could be misread as a flag or break
// the argv passed to tshark/tcpdump (leading '-', whitespace, control chars).
func validateIface(iface string) error {
	if iface == "" || iface[0] == '-' {
		return fmt.Errorf("invalid interface name %q", iface)
	}
	for _, r := range iface {
		if r <= 0x20 || r == 0x7f {
			return fmt.Errorf("invalid interface name %q", iface)
		}
	}
	return nil
}

// netnsExists reports whether a named network namespace exists on this host.
func netnsExists(name string) bool {
	if name == "" {
		return false
	}
	if _, err := os.Stat("/run/netns/" + name); err == nil {
		return true
	}
	_, err := os.Stat("/var/run/netns/" + name)
	return err == nil
}

// Start creates and begins a new capture session. A non-empty, existing netns
// runs the capture inside that namespace so it sees a running network's AP-side
// client traffic; otherwise it runs on the host. onExit, which may be nil, runs
// in a goroutine on process exit with the final packet count.
func Start(id, iface, netns string, layer Layer, filter string, onExit func(packetCount int)) (*Session, error) {
	if err := ValidateBPFFilter(filter); err != nil {
		return nil, err
	}
	if err := validateIface(iface); err != nil {
		return nil, err
	}

	mu.Lock()
	defer mu.Unlock()

	if _, exists := sessions[id]; exists {
		return nil, fmt.Errorf("capture %s already running", id)
	}

	if err := os.MkdirAll(CaptureDir, 0750); err != nil {
		return nil, fmt.Errorf("mkdir capture dir: %w", err)
	}

	pcapPath := filepath.Join(CaptureDir, id+".pcapng")
	ctx, cancel := context.WithCancel(context.Background())

	// Prefer tshark; fall back to tcpdump.
	var name string
	var args []string
	if bin, lookErr := exec.LookPath("tshark"); lookErr == nil {
		name = bin
		args = []string{"-i", iface, "-w", pcapPath, "-F", "pcapng", "-q"}
		if layer == LayerWireless {
			args = append(args, "-I") // monitor mode
		}
		if filter != "" {
			args = append(args, "-f", filter)
		}
	} else {
		name = "tcpdump"
		args = []string{"-i", iface, "-w", pcapPath}
		if layer == LayerWireless {
			args = append(args, "-y", "IEEE802_11")
		}
		if filter != "" {
			args = append(args, "--", filter)
		}
	}

	// Enter the namespace if it exists; the shared filesystem still lands the
	// pcap on the host under CaptureDir.
	if netnsExists(netns) {
		args = append([]string{"netns", "exec", netns, name}, args...)
		name = "ip"
	}

	cmd := exec.CommandContext(ctx, name, args...)

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("capture start: %w", err)
	}

	session := &Session{
		ID:        id,
		Interface: iface,
		Layer:     layer,
		Filter:    filter,
		FilePath:  pcapPath,
		cmd:       cmd,
		cancel:    cancel,
		onExit:    onExit,
		finished:  make(chan int, 1),
	}

	sessions[id] = session

	go func() {
		_ = cmd.Wait()
		count := PacketCount(pcapPath)
		mu.Lock()
		delete(sessions, id)
		mu.Unlock()
		session.finished <- count // buffered, so this never blocks
		if onExit != nil {
			onExit(count)
		}
	}()

	return session, nil
}

// PacketCount returns the number of packets in a pcap file via capinfos, or 0 if
// the file is missing, capinfos is unavailable, or parsing fails.
func PacketCount(pcapPath string) int {
	if _, err := os.Stat(pcapPath); err != nil {
		return 0
	}
	out, err := exec.Command("capinfos", "-c", "-M", pcapPath).Output()
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Number of packets") {
			continue
		}
		// Format: "Number of packets:   42"
		idx := strings.LastIndex(line, ":")
		if idx == -1 || idx+1 >= len(line) {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimSpace(line[idx+1:])); err == nil {
			return n
		}
	}
	return 0
}

// Stop ends a running capture session (fire-and-forget).
func Stop(id string) error {
	_, err := StopAndWait(id, 0)
	return err
}

// StopAndWait stops a running capture and waits up to timeout for the process to
// exit and the pcap to be finalized, returning the final packet count. A zero
// timeout does not wait.
func StopAndWait(id string, timeout time.Duration) (int, error) {
	mu.Lock()
	session, exists := sessions[id]
	mu.Unlock()

	if !exists {
		return 0, fmt.Errorf("capture %s not running", id)
	}

	session.cancel()
	if timeout <= 0 {
		return 0, nil
	}
	select {
	case count := <-session.finished:
		return count, nil
	case <-time.After(timeout):
		// Process did not report in time; read the count off disk.
		log.Printf("[capture] %s stop wait timed out, reading count from disk", id)
		return PacketCount(session.FilePath), nil
	}
}
