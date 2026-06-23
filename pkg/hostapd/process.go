// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package hostapd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Client represents a connected wireless client.
type Client struct {
	MAC    string `json:"mac"`
	Signal int    `json:"signal"`
}

// Process manages a running hostapd instance.
type Process struct {
	cmd      *exec.Cmd
	confPath string
	logCh    chan string
	cancel   context.CancelFunc
	mu       sync.RWMutex
	alive    bool

	// done is closed when Start's supervisor goroutine observes hostapd's exit.
	// Stop() waits on this instead of calling cmd.Wait() a second time, which
	// deadlocks (concurrent Wait() on the same Cmd is not allowed).
	done chan struct{}
}

// Start launches hostapd with the given config file, inside netns if provided.
// binary is the hostapd executable to run; empty falls back to "hostapd" from PATH.
func Start(ctx context.Context, confPath string, netnsName string, binary string) (*Process, error) {
	ctx, cancel := context.WithCancel(ctx)

	if binary == "" {
		binary = "hostapd"
	}

	var cmd *exec.Cmd
	if netnsName != "" {
		cmd = exec.CommandContext(ctx, "ip", "netns", "exec", netnsName, binary, "-d", confPath)
	} else {
		cmd = exec.CommandContext(ctx, binary, "-d", confPath)
	}

	p := &Process{
		cmd:      cmd,
		confPath: confPath,
		logCh:    make(chan string, 512),
		cancel:   cancel,
		alive:    true,
		done:     make(chan struct{}),
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("hostapd stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("hostapd stderr pipe: %w", err)
	}

	// Capture every line so the failure path can report the real cause, which
	// appears earlier than hostapd's teardown chain. The WaitGroup lets the
	// failure path wait for both readers to hit EOF before inspecting.
	var (
		capMu    sync.Mutex
		captured []string
		scanWg   sync.WaitGroup
	)
	scanWg.Add(2)
	scan := func(r io.Reader) {
		defer scanWg.Done()
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			capMu.Lock()
			captured = append(captured, line)
			capMu.Unlock()
			select {
			case p.logCh <- line:
			default:
			}
		}
	}
	go scan(stdout)
	go scan(stderr)

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("hostapd start: %w", err)
	}

	exitCh := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		p.mu.Lock()
		p.alive = false
		p.mu.Unlock()
		close(p.done)
		exitCh <- err
	}()

	// Wait briefly to confirm the process stabilized.
	select {
	case <-exitCh:
		cancel()
		scanWg.Wait() // let both readers finish so we inspect the full output

		capMu.Lock()
		lines := append([]string(nil), captured...)
		capMu.Unlock()

		// Write the full trace to a file since journald drops large multi-line messages.
		const failLog = "/tmp/tala-hostapd-fail.log"
		if err := os.WriteFile(failLog, []byte(strings.Join(lines, "\n")), 0o600); err == nil {
			log.Printf("[hostapd] start failed (%d lines captured); full output: %s", len(lines), failLog)
		} else {
			log.Printf("[hostapd] start failed (%d lines captured)", len(lines))
		}

		return nil, fmt.Errorf("hostapd failed to start: %s", summarizeHostapdFailure(lines))
	case <-time.After(500 * time.Millisecond):
		return p, nil
	}
}

// summarizeHostapdFailure extracts the first informative line from hostapd's output (the real error precedes its teardown chain), plus the tail for context.
func summarizeHostapdFailure(lines []string) string {
	var trimmed []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			trimmed = append(trimmed, l)
		}
	}
	if len(trimmed) == 0 {
		return "no output (process exited immediately)"
	}
	signals := []string{
		"could not", "failed to", "interface initialization failed",
		"unable to", "no such", "invalid", "not allowed", "errno",
		"set_freq", "frequency", "channel", "rfkill", "busy",
		"eperm", "ebusy", "einval", "unsupported", "operation not",
	}
	var fatal string
	for _, l := range trimmed {
		ll := strings.ToLower(l)
		for _, s := range signals {
			if strings.Contains(ll, s) {
				fatal = l
				break
			}
		}
		if fatal != "" {
			break
		}
	}
	start := 0
	if len(trimmed) > 3 {
		start = len(trimmed) - 3
	}
	tail := strings.Join(trimmed[start:], " | ")
	if fatal != "" && !strings.Contains(tail, fatal) {
		return fatal + " || " + tail
	}
	return tail
}

// Stop terminates hostapd and blocks until it exits (up to 5s), then escalates to SIGKILL. Waits on p.done rather than calling cmd.Wait() a second time (which would deadlock).
func (p *Process) Stop() error {
	p.cancel()
	select {
	case <-p.done:
		return nil
	case <-time.After(5 * time.Second):
		// SIGKILL anything that ignored cancel(); the supervisor closes p.done shortly after.
		if p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}
		select {
		case <-p.done:
		case <-time.After(2 * time.Second):
		}
		return nil
	}
}

// IsRunning returns whether the process is alive.
func (p *Process) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.alive
}

// LogCh returns the log output channel.
func (p *Process) LogCh() <-chan string {
	return p.logCh
}

// Clients queries connected clients via hostapd_cli.
func (p *Process) Clients() []Client {
	out, err := exec.Command("hostapd_cli", "-p", "/var/run/hostapd", "all_sta").Output()
	if err != nil {
		return nil
	}
	return parseStaOutput(string(out))
}

func parseStaOutput(output string) []Client {
	var clients []Client
	var current *Client
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if isMACAddress(line) {
			if current != nil {
				clients = append(clients, *current)
			}
			current = &Client{MAC: line}
		} else if current != nil && strings.HasPrefix(line, "signal=") {
			// Signal is optional; on a malformed line it simply stays zero.
			_, _ = fmt.Sscanf(strings.TrimPrefix(line, "signal="), "%d", &current.Signal)
		}
	}
	if current != nil {
		clients = append(clients, *current)
	}
	return clients
}

func isMACAddress(s string) bool {
	if len(s) != 17 {
		return false
	}
	for i, c := range s {
		if i%3 == 2 {
			if c != ':' {
				return false
			}
		} else {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}
