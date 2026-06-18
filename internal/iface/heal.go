// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package iface

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// InterfaceType returns the iw operating mode of an interface ("managed", "AP",
// "monitor", ...), or "" if it cannot be read. An idle radio reads "managed";
// anything else before an AP start means a prior run left it dirty.
func InterfaceType(ifName string) string {
	out, err := exec.Command("iw", "dev", ifName, "info").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "type ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "type"))
		}
	}
	return ""
}

// HealAdapter recovers a wedged USB Wi-Fi adapter by unbinding and rebinding its
// USB device, forcing a clean driver re-probe and firmware reload. MediaTek
// radios in particular can get stuck after a rapid stop/start and stop beaconing
// until re-probed. The interface keeps its name (same MAC) across the rebind.
func HealAdapter(ifName string) error {
	link, err := os.Readlink(filepath.Join("/sys/class/net", ifName, "device"))
	if err != nil {
		return fmt.Errorf("no USB device for %s: %w", ifName, err)
	}
	// e.g. ".../3-1:1.0" -> USB device id "3-1" (strip the interface suffix).
	usbDev := filepath.Base(link)
	if i := strings.Index(usbDev, ":"); i > 0 {
		usbDev = usbDev[:i]
	}

	const drv = "/sys/bus/usb/drivers/usb"
	if err := os.WriteFile(filepath.Join(drv, "unbind"), []byte(usbDev), 0o200); err != nil {
		return fmt.Errorf("unbind %s: %w", usbDev, err)
	}
	time.Sleep(1 * time.Second)
	if err := os.WriteFile(filepath.Join(drv, "bind"), []byte(usbDev), 0o200); err != nil {
		return fmt.Errorf("bind %s: %w", usbDev, err)
	}

	// Wait for the interface to re-enumerate, then let the driver settle.
	for i := 0; i < 16; i++ {
		if _, err := os.Stat(filepath.Join("/sys/class/net", ifName)); err == nil {
			time.Sleep(1500 * time.Millisecond)
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("%s did not reappear after USB rebind", ifName)
}
