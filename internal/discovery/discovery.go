// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. See the LICENSE file.

// Package discovery advertises this Tala WTE instance on the LAN over mDNS and
// browses for other instances, so a leader can find potential pack members (and
// other leaders) without knowing their addresses in advance - which also handles
// fresh installs and hosts whose DHCP address changes.
package discovery

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"
)

const (
	service = "_tala-wte._tcp"
	domain  = "local."
)

// Peer is a Tala WTE instance discovered on the LAN.
type Peer struct {
	Name    string   `json:"name"`
	Host    string   `json:"host"`
	Address string   `json:"address"` // best host:port to reach the instance
	IPs     []string `json:"ips"`
	Port    int      `json:"port"`
	Role    string   `json:"role"` // "leader" or "member"
	Version string   `json:"version"`
}

// Advertise registers this instance as a _tala-wte._tcp service so other Tala
// instances on the LAN can find it. Close the returned server to stop.
func Advertise(instance, role, version string, port int) (*zeroconf.Server, error) {
	txt := []string{"role=" + role, "version=" + version, "name=" + instance}
	return zeroconf.Register(instance, service, domain, port, txt, nil)
}

// Browse looks for other Tala WTE instances on the LAN for the given duration and
// returns what it found. It is safe to call repeatedly.
func Browse(timeout time.Duration) ([]Peer, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, err
	}
	entries := make(chan *zeroconf.ServiceEntry)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := resolver.Browse(ctx, service, domain, entries); err != nil {
		return nil, err
	}

	var peers []Peer
	for e := range entries {
		p := Peer{
			Name: strings.TrimSuffix(e.Instance, "."),
			Host: strings.TrimSuffix(e.HostName, "."),
			Port: e.Port,
		}
		for _, ip := range e.AddrIPv4 {
			p.IPs = append(p.IPs, ip.String())
		}
		if len(p.IPs) > 0 {
			p.Address = fmt.Sprintf("%s:%d", p.IPs[0], e.Port)
		}
		for _, t := range e.Text {
			if v, ok := strings.CutPrefix(t, "role="); ok {
				p.Role = v
			} else if v, ok := strings.CutPrefix(t, "version="); ok {
				p.Version = v
			} else if v, ok := strings.CutPrefix(t, "name="); ok && p.Name == "" {
				p.Name = v
			}
		}
		peers = append(peers, p)
	}
	return peers, nil
}
