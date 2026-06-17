// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

package client

import (
	"context"
	"math/rand"
	"net"
	"strings"
	"syscall"
	"time"
)

// hostnames responder commonly poisons; queries for these from the client give a
// trainee running responder.py on the wireless network something to poison.
var baitNames = []string{
	"wpad", "fileserver", "fileshare", "intranet", "sharepoint",
	"printer01", "dc01", "backup", "helpdesk", "sql01",
}

// genDomain emits Windows-style name-resolution chatter (LLMNR, mDNS, NBT-NS) for
// plausible internal hostnames. These broadcast/multicast lookups are exactly what
// LLMNR/NBT-NS poisoners (responder.py, Inveigh) intercept, so they make
// poisoning demonstrable on the wireless network. The follow-on SMB/NTLM auth
// that leaks a NetNTLMv2 hash is a separate, heavier step and is not done here.
func (a *Agent) genDomain(ctx context.Context, opts TrafficOptions) {
	names := append([]string{}, baitNames...)
	for _, d := range opts.Domains { // first label of any supplied domain
		if label := strings.SplitN(d, ".", 2)[0]; label != "" {
			names = append(names, label)
		}
	}
	for {
		if ctx.Err() != nil {
			return
		}
		name := names[rand.Intn(len(names))]
		if sendLLMNR(name) {
			a.inc(1, 0)
		}
		if sendMDNS(name) {
			a.inc(1, 0)
		}
		if sendNBTNS(name) {
			a.inc(1, 0)
		}
		sleepJitter(ctx, 3*time.Second, 8*time.Second)
	}
}

// buildDNSQuery builds a standard DNS A-record query for a name (used for LLMNR
// and mDNS, which share the DNS wire format).
func buildDNSQuery(name string) []byte {
	b := []byte{0x13, 0x37, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	for _, label := range strings.Split(name, ".") {
		b = append(b, byte(len(label)))
		b = append(b, []byte(label)...)
	}
	b = append(b, 0x00)       // end of name
	b = append(b, 0x00, 0x01) // type A
	b = append(b, 0x00, 0x01) // class IN
	return b
}

func sendLLMNR(name string) bool {
	return sendUDP("224.0.0.252:5355", buildDNSQuery(name), false)
}

func sendMDNS(name string) bool {
	return sendUDP("224.0.0.251:5353", buildDNSQuery(name+".local"), false)
}

// sendNBTNS sends a NetBIOS name-service query (UDP 137 broadcast). Responder
// poisons NBT-NS as well as LLMNR.
func sendNBTNS(name string) bool {
	pkt := []byte{0x13, 0x37, 0x01, 0x10, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	pkt = append(pkt, 0x20) // name length (always 0x20 after L1 encoding)
	pkt = append(pkt, encodeNetBIOSName(name)...)
	pkt = append(pkt, 0x00)       // null terminator
	pkt = append(pkt, 0x00, 0x20) // type NB
	pkt = append(pkt, 0x00, 0x01) // class IN
	return sendUDP("255.255.255.255:137", pkt, true)
}

// encodeNetBIOSName applies NetBIOS first-level (half-ASCII) encoding: each byte
// of the 16-char padded name becomes two nibble-letters (A..P).
func encodeNetBIOSName(name string) []byte {
	n := strings.ToUpper(name)
	if len(n) > 15 {
		n = n[:15]
	}
	padded := n + strings.Repeat(" ", 15-len(n)) + "\x00"
	out := make([]byte, 0, 32)
	for i := 0; i < 16; i++ {
		c := padded[i]
		out = append(out, 'A'+(c>>4), 'A'+(c&0x0f))
	}
	return out
}

// sendUDP fires a single best-effort UDP datagram. broadcast enables SO_BROADCAST
// for the NBT-NS limited-broadcast case.
func sendUDP(addr string, payload []byte, broadcast bool) bool {
	d := net.Dialer{Timeout: 2 * time.Second}
	if broadcast {
		d.Control = func(_, _ string, c syscall.RawConn) error {
			var serr error
			if err := c.Control(func(fd uintptr) {
				serr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
			}); err != nil {
				return err
			}
			return serr
		}
	}
	conn, err := d.Dial("udp4", addr)
	if err != nil {
		return false
	}
	defer conn.Close()
	_, err = conn.Write(payload)
	return err == nil
}
