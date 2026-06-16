// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

// Package routing manages NAT/iptables rules and dynamic veth tunneling for internet passthrough.
package routing

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
)

// EnableForwarding enables IPv4 packet forwarding globally.
func EnableForwarding() error {
	return os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644)
}

// AssignIP assigns an IP to an interface (`ip addr add ip dev iface`)
func AssignIP(iface, ip string) error {
	cmd := exec.Command("ip", "addr", "add", ip, "dev", iface)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ip addr add: %s: %w", out, err)
	}
	return nil
}

// VethTopology defines an active veth bridge
type VethTopology struct {
	ID        string
	HostIface string
	PeerIface string
	HostIP    string
	PeerIP    string
	Uplink    string
	Subnet    string
	Octet     int // allocated subnet octet for pool tracking
}

var (
	octetMu    sync.Mutex
	usedOctets = make(map[int]bool)
)

// allocateOctet reserves a unique subnet octet based on the session ID hash,
// incrementing to find a free one if the preferred octet is already in use.
func allocateOctet(id string) int {
	octetMu.Lock()
	defer octetMu.Unlock()

	hash := sha256.Sum256([]byte(id))
	octet := 10 + (int(hash[0]) % 200)

	start := octet
	for usedOctets[octet] {
		octet++
		if octet > 210 {
			octet = 10
		}
		if octet == start {
			break
		}
	}
	usedOctets[octet] = true
	return octet
}

// releaseOctet returns an octet to the pool for reuse.
func releaseOctet(octet int) {
	octetMu.Lock()
	defer octetMu.Unlock()
	delete(usedOctets, octet)
}

// SetupVethTunnel dynamically spins up a virtual ethernet pair between the host and the locked namespace,
// orchestrating the routing table and iptables Double-NAT automatically.
func SetupVethTunnel(id, nsName, uplinkIface string) (*VethTopology, error) {
	if err := EnableForwarding(); err != nil {
		return nil, fmt.Errorf("enable forwarding: %w", err)
	}

	octet := allocateOctet(id)

	shortID := id
	if len(id) > 4 {
		shortID = id[:4]
	}

	topology := &VethTopology{
		ID:        id,
		HostIface: fmt.Sprintf("vth-%s", shortID),
		PeerIface: fmt.Sprintf("vth-%s-p", shortID),
		HostIP:    fmt.Sprintf("192.168.%d.1", octet),
		PeerIP:    fmt.Sprintf("192.168.%d.2", octet),
		Subnet:    fmt.Sprintf("192.168.%d.0/24", octet),
		Uplink:    uplinkIface,
		Octet:     octet,
	}

	// rollback cleans up resources created by earlier steps on failure.
	rollback := func() {
		releaseOctet(octet)
		_ = exec.Command("ip", "link", "delete", topology.HostIface).Run()
	}

	_ = exec.Command("ip", "link", "delete", topology.HostIface).Run()
	if out, err := exec.Command("ip", "link", "add", topology.HostIface, "type", "veth", "peer", "name", topology.PeerIface).CombinedOutput(); err != nil {
		releaseOctet(octet)
		return nil, fmt.Errorf("add veth: %s: %w", out, err)
	}

	if out, err := exec.Command("ip", "link", "set", topology.PeerIface, "netns", nsName).CombinedOutput(); err != nil {
		rollback()
		return nil, fmt.Errorf("move veth peer: %s: %w", out, err)
	}

	if err := AssignIP(topology.HostIface, topology.HostIP+"/24"); err != nil {
		rollback()
		return nil, fmt.Errorf("assign host IP: %w", err)
	}
	if out, err := exec.Command("ip", "link", "set", topology.HostIface, "up").CombinedOutput(); err != nil {
		rollback()
		return nil, fmt.Errorf("bring up host veth: %s: %w", out, err)
	}

	if out, err := exec.Command("ip", "netns", "exec", nsName, "ip", "addr", "add", topology.PeerIP+"/24", "dev", topology.PeerIface).CombinedOutput(); err != nil {
		rollback()
		return nil, fmt.Errorf("assign peer IP: %s: %w", out, err)
	}
	if out, err := exec.Command("ip", "netns", "exec", nsName, "ip", "link", "set", topology.PeerIface, "up").CombinedOutput(); err != nil {
		rollback()
		return nil, fmt.Errorf("bring up peer veth: %s: %w", out, err)
	}

	if out, err := exec.Command("ip", "netns", "exec", nsName, "ip", "route", "add", "default", "via", topology.HostIP).CombinedOutput(); err != nil {
		log.Printf("[routing] default route via %s in %s failed (may already exist): %s: %v", topology.HostIP, nsName, out, err)
	}

	// ip_forward is per-netns, so the host's setting does not apply; without this
	// the namespace gives clients an IP and gateway but never routes them out.
	if out, err := exec.Command("ip", "netns", "exec", nsName, "sysctl", "-w", "net.ipv4.ip_forward=1").CombinedOutput(); err != nil {
		log.Printf("[routing] enable ip_forward in %s failed: %s: %v", nsName, out, err)
	}

	// Host-side SNAT masquerade for the subnet out to the uplink.
	if err := iptables("-t", "nat", "-A", "POSTROUTING", "-s", topology.Subnet, "-o", topology.Uplink, "-j", "MASQUERADE"); err != nil {
		rollback()
		return nil, fmt.Errorf("host NAT: %w", err)
	}

	if out, err := exec.Command("ip", "netns", "exec", nsName, "iptables", "-t", "nat", "-A", "POSTROUTING", "-o", topology.PeerIface, "-j", "MASQUERADE").CombinedOutput(); err != nil {
		_ = iptables("-t", "nat", "-D", "POSTROUTING", "-s", topology.Subnet, "-o", topology.Uplink, "-j", "MASQUERADE")
		rollback()
		return nil, fmt.Errorf("namespace NAT: %s: %w", out, err)
	}

	return topology, nil
}

// TeardownVethTunnel gracefully destroys the veth interface and drops NAT rules
func TeardownVethTunnel(topology *VethTopology) {
	if topology == nil {
		return
	}
	_ = iptables("-t", "nat", "-D", "POSTROUTING", "-s", topology.Subnet, "-o", topology.Uplink, "-j", "MASQUERADE")
	_ = exec.Command("ip", "link", "delete", topology.HostIface).Run()
	releaseOctet(topology.Octet)
}

func iptables(args ...string) error {
	cmd := exec.Command("iptables", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("iptables %v: %s: %w", args, out, err)
	}
	return nil
}
