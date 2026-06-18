// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package routing

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"text/template"
	"time"
)

type DNSMasqConfig struct {
	Interface string
	GatewayIP string
	DHCPRange string
	SessionID string
}

const dnsmasqTmpl = `
# dnsmasq config for Tala WTE {{.SessionID}}
interface={{.Interface}}
dhcp-range={{.DHCPRange}},12h
dhcp-option=3,{{.GatewayIP}}
dhcp-option=6,{{.GatewayIP}}
server=8.8.8.8
server=1.1.1.1
`

func GenerateDNSMasqConfig(c DNSMasqConfig) (string, error) {
	tmpl, err := template.New("dnsmasq").Parse(dnsmasqTmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, c); err != nil {
		return "", err
	}

	path := fmt.Sprintf("/tmp/dnsmasq-%s.conf", c.SessionID)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

type DNSMasqProcess struct {
	cmd *exec.Cmd
}

func StartDNSMasq(confPath string, netnsName string, onLog func(string)) (*DNSMasqProcess, error) {
	var cmd *exec.Cmd
	if netnsName != "" {
		cmd = exec.Command("ip", "netns", "exec", netnsName, "dnsmasq", "-C", confPath, "-d")
	} else {
		cmd = exec.Command("dnsmasq", "-C", confPath, "-d") // -d keeps it in foreground
	}
	// Capture dnsmasq's foreground output so the per-network live log can show
	// DHCP and lease activity.
	if onLog != nil {
		if stdout, err := cmd.StdoutPipe(); err == nil {
			go scanDNSMasqLines(stdout, onLog)
		}
		if stderr, err := cmd.StderrPipe(); err == nil {
			go scanDNSMasqLines(stderr, onLog)
		}
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &DNSMasqProcess{cmd: cmd}, nil
}

func scanDNSMasqLines(r io.Reader, onLog func(string)) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		onLog(s.Text())
	}
}

func (p *DNSMasqProcess) Stop() error {
	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	_ = p.cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan error, 1)
	go func() { done <- p.cmd.Wait() }()
	select {
	case <-done:
		return nil
	case <-time.After(3 * time.Second):
		return p.cmd.Process.Kill()
	}
}
