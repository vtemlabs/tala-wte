// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

// Read-only packet-list and per-packet detail views of a saved pcap via tshark,
// narrowed by a Wireshark display filter (distinct from the BPF capture filter).
package capture

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// PacketRow is one row of the packet list, mirroring Wireshark's columns.
type PacketRow struct {
	No       int    `json:"no"`
	Time     string `json:"time"`
	Source   string `json:"source"`
	Dest     string `json:"dest"`
	Protocol string `json:"protocol"`
	Length   int    `json:"length"`
	Info     string `json:"info"`
}

// ValidateDisplayFilter guards a Wireshark display filter. Passed as a single
// argv element (no shell), so the only risks are control characters and a
// leading dash that tshark would read as a flag; tshark rejects bad syntax.
func ValidateDisplayFilter(f string) error {
	if f == "" {
		return nil
	}
	if strings.ContainsAny(f, "\n\r\x00") {
		return fmt.Errorf("invalid display filter: control characters")
	}
	if f[0] == '-' {
		return fmt.Errorf("invalid display filter")
	}
	return nil
}

// Packets returns up to limit packet rows from a capture, optionally narrowed by
// a display filter. The second return reports truncation at the limit.
func Packets(id, displayFilter string, limit int) ([]PacketRow, bool, error) {
	pcap := PcapPath(id)
	if _, err := os.Stat(pcap); err != nil {
		return nil, false, err
	}
	if err := ValidateDisplayFilter(displayFilter); err != nil {
		return nil, false, err
	}
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}

	args := []string{
		"-r", pcap, "-T", "fields",
		"-e", "frame.number",
		"-e", "frame.time_relative",
		"-e", "_ws.col.Source",
		"-e", "_ws.col.Destination",
		"-e", "_ws.col.Protocol",
		"-e", "frame.len",
		"-e", "_ws.col.Info",
		"-c", strconv.Itoa(limit + 1),
	} // one extra to detect truncation
	if displayFilter != "" {
		args = append(args, "-Y", displayFilter)
	}

	// Keep stdout (rows) and stderr (root notice, filter errors) separate so the
	// warning never pollutes the parsed rows.
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("tshark", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, false, fmt.Errorf("%s", strings.TrimSpace(lastLine(stderr.String())))
	}

	var rows []PacketRow
	for _, line := range strings.Split(stdout.String(), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Split(line, "\t")
		row := PacketRow{}
		if len(f) > 0 {
			row.No, _ = strconv.Atoi(f[0])
		}
		if len(f) > 1 {
			row.Time = f[1]
		}
		if len(f) > 2 {
			row.Source = f[2]
		}
		if len(f) > 3 {
			row.Dest = f[3]
		}
		if len(f) > 4 {
			row.Protocol = f[4]
		}
		if len(f) > 5 {
			row.Length, _ = strconv.Atoi(f[5])
		}
		if len(f) > 6 {
			row.Info = strings.Join(f[6:], " ")
		}
		rows = append(rows, row)
	}

	truncated := false
	if len(rows) > limit {
		rows = rows[:limit]
		truncated = true
	}
	return rows, truncated, nil
}

// PacketDetail returns tshark's full verbose dissection for a single frame.
func PacketDetail(id string, frameNumber int) (string, error) {
	pcap := PcapPath(id)
	if _, err := os.Stat(pcap); err != nil {
		return "", err
	}
	if frameNumber <= 0 {
		return "", fmt.Errorf("invalid frame number")
	}
	out, err := exec.Command("tshark", "-r", pcap,
		"-Y", fmt.Sprintf("frame.number==%d", frameNumber), "-V").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func lastLine(s string) string {
	s = strings.TrimRight(s, "\n")
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return s[i+1:]
	}
	return s
}
