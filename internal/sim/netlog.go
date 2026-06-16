// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

package sim

// Per-network ring buffer of live activity surfaced via GET /api/wte/networks/{id}/logs.

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/vtemlabs/tala-wte/pkg/hostapd"
)

const netLogCap = 600

type netLogBuf struct {
	mu    sync.Mutex
	lines []string
}

var (
	netLogMu sync.Mutex
	netLogs  = map[string]*netLogBuf{}
)

func netLogGet(id string) *netLogBuf {
	netLogMu.Lock()
	defer netLogMu.Unlock()
	b := netLogs[id]
	if b == nil {
		b = &netLogBuf{}
		netLogs[id] = b
	}
	return b
}

// netLogAppend adds a timestamped line to the network's ring buffer.
func netLogAppend(id, line string) {
	b := netLogGet(id)
	stamped := time.Now().Format("15:04:05") + " " + line
	b.mu.Lock()
	b.lines = append(b.lines, stamped)
	if len(b.lines) > netLogCap {
		b.lines = b.lines[len(b.lines)-netLogCap:]
	}
	b.mu.Unlock()
}

// netLogf is a printf-style convenience wrapper.
func netLogf(id, format string, args ...any) {
	netLogAppend(id, fmt.Sprintf(format, args...))
}

// netLogHistory returns a copy of the buffered lines.
func netLogHistory(id string) []string {
	b := netLogGet(id)
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, len(b.lines))
	copy(out, b.lines)
	return out
}

// netLogReset clears a network's buffer (called when it (re)starts).
func netLogReset(id string) {
	netLogMu.Lock()
	delete(netLogs, id)
	netLogMu.Unlock()
}

// drainHostapdLog copies interesting hostapd output into the live log until the process exits.
// hostapd's logCh is never closed, so poll IsRunning to exit rather than leak a blocked goroutine.
func drainHostapdLog(id string, proc *hostapd.Process) {
	ch := proc.LogCh()
	for {
		select {
		case line, ok := <-ch:
			if !ok {
				return
			}
			if hostapdInteresting(line) {
				netLogAppend(id, "[hostapd] "+line)
			}
		case <-time.After(3 * time.Second):
			if !proc.IsRunning() {
				for {
					select {
					case line := <-ch:
						if hostapdInteresting(line) {
							netLogAppend(id, "[hostapd] "+line)
						}
					default:
						return
					}
				}
			}
		}
	}
}

// hostapdInteresting filters hostapd's verbose -d output down to client/auth/handshake/AP events worth surfacing.
func hostapdInteresting(l string) bool {
	ll := strings.ToLower(l)
	for _, p := range []string{
		"ap-sta", "ap-enabled", "ap-disabled", " sta ", "associat", "authent",
		"deauth", "disassoc", "eapol", "handshake", "4-way", "wpa:", "pairwise",
		"interface state", "radius", "eap ", "fail", "error", "denied", "reject",
	} {
		if strings.Contains(ll, p) {
			return true
		}
	}
	return false
}
