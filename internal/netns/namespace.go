// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

// Package netns manages Linux network namespaces for AP isolation.
package netns

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/vishvananda/netns"
)

// SimNamespace wraps a Linux network namespace for a simulated AP.
type SimNamespace struct {
	Name   string
	handle netns.NsHandle
}

// Create creates a new named network namespace.
func Create(name string) (*SimNamespace, error) {
	if err := run("ip", "netns", "add", name); err != nil {
		return nil, fmt.Errorf("ip netns add %s: %w", name, err)
	}
	handle, err := netns.GetFromName(name)
	if err != nil {
		return nil, fmt.Errorf("get netns %s: %w", name, err)
	}
	return &SimNamespace{Name: name, handle: handle}, nil
}

// Delete removes the network namespace.
func (n *SimNamespace) Delete() error {
	_ = n.handle.Close()
	return run("ip", "netns", "del", n.Name)
}

// Exec runs fn inside the namespace, locking the OS thread as required.
func (n *SimNamespace) Exec(fn func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	origin, err := netns.Get()
	if err != nil {
		return fmt.Errorf("get origin netns: %w", err)
	}
	defer netns.Set(origin)

	if err := netns.Set(n.handle); err != nil {
		return fmt.Errorf("set netns %s: %w", n.Name, err)
	}
	return fn()
}

// MoveInterface moves a wireless PHY to the namespace.
func (n *SimNamespace) MoveInterface(phyName string) error {
	return run("iw", "phy", phyName, "set", "netns", "name", n.Name)
}

// SetupLoopback brings up the loopback interface inside the namespace.
func (n *SimNamespace) SetupLoopback() error {
	return run("ip", "netns", "exec", n.Name, "ip", "link", "set", "lo", "up")
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %v: %s: %w", name, args, out, err)
	}
	return nil
}
