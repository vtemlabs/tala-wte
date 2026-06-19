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

// replugHint is appended to heal failures: a software USB reset cannot recover a
// device whose virtual xHCI is provided by a hypervisor, so the operator must
// physically replug (or re-attach the device in the hypervisor).
const replugHint = "; if this is a VM the hypervisor blocks software USB resets - physically replug the adapter, or detach/attach it in the hypervisor"

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

	if err := usbRebind(usbDev); err != nil {
		return err
	}

	// Wait for the interface to re-enumerate, then let the driver settle.
	for i := 0; i < 16; i++ {
		if _, err := os.Stat(filepath.Join("/sys/class/net", ifName)); err == nil {
			time.Sleep(1500 * time.Millisecond)
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("%s did not reappear after USB rebind%s", ifName, replugHint)
}

// HealUSBDevice recovers an adapter that enumerated but never initialized (no
// ieee80211 phy) - the firmware-init wedge common after a cold boot or on a
// USB-passthrough VM. It rebinds the USB device by its sysfs path (e.g. "2-3")
// and waits for a phy to appear, which means the driver bound and firmware loaded.
func HealUSBDevice(usbPath string) error {
	devDir := filepath.Join("/sys/bus/usb/devices", usbPath)
	if _, err := os.Stat(devDir); err != nil {
		return fmt.Errorf("usb device %s is not present", usbPath)
	}
	if err := usbRebind(usbPath); err != nil {
		return err
	}
	// A successful firmware load creates a phy in ~2s; a repeat wedge exhausts the
	// kernel's patch-semaphore retries in ~20s. Poll up to ~24s.
	for i := 0; i < 24; i++ {
		if usbDeviceHasPhy(devDir) {
			time.Sleep(1500 * time.Millisecond)
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("adapter at %s did not initialize after USB reset%s", usbPath, replugHint)
}

// usbRebind unbinds then rebinds a USB device from the usb core driver, forcing a
// clean re-probe. It refuses an mt76 NPU-coupled device on an unpatched kernel,
// where the unbind teardown can hard-lock the host.
func usbRebind(usbDev string) error {
	if risk, msg := mt76NPUHardlockRisk(usbDev); risk {
		return fmt.Errorf("%s", msg)
	}
	const drv = "/sys/bus/usb/drivers/usb"
	// Keep the link awake so USB autosuspend can't abort the firmware download.
	_ = os.WriteFile(filepath.Join("/sys/bus/usb/devices", usbDev, "power", "control"), []byte("on"), 0o200)
	if err := os.WriteFile(filepath.Join(drv, "unbind"), []byte(usbDev), 0o200); err != nil {
		return fmt.Errorf("unbind %s: %w", usbDev, err)
	}
	time.Sleep(1 * time.Second)
	if err := os.WriteFile(filepath.Join(drv, "bind"), []byte(usbDev), 0o200); err != nil {
		return fmt.Errorf("bind %s: %w", usbDev, err)
	}
	return nil
}

// mt76NPUHardlockRisk reports whether unbinding the USB device could hard-lock
// the host via the mt76 NPU-teardown bug (Linux 7.0+). When the mt76 module is
// NPU-coupled (depends on airoha_npu), unbinding any mt76-family adapter drives
// mt76_free_device -> mt76_npu_deinit, which oopses. The operator clears this by
// rebuilding mt76 without the NPU (or applying the fix and touching the marker).
func mt76NPUHardlockRisk(usbDev string) (bool, string) {
	if !usbDeviceUsesMT76(usbDev) {
		return false, ""
	}
	if _, err := os.Stat("/var/lib/tala-wte/mt76-npu-safe"); err == nil {
		return false, "" // operator has marked this kernel safe
	}
	out, err := exec.Command("modinfo", "-F", "depends", "mt76").Output()
	if err != nil {
		return false, "" // can't determine; do not block
	}
	for _, dep := range strings.Split(strings.TrimSpace(string(out)), ",") {
		if strings.TrimSpace(dep) == "airoha_npu" {
			return true, "this MediaTek adapter uses the mt76 driver, which is NPU-coupled on this kernel (Linux 7.0 teardown bug); a software USB reset could hard-lock the host. Physically replug the adapter instead, or apply the mt76 NPU fix and 'touch /var/lib/tala-wte/mt76-npu-safe' to enable healing."
		}
	}
	return false, ""
}

// usbDeviceUsesMT76 reports whether the USB device is (or, if its probe failed,
// likely is) a MediaTek mt76-family adapter - the family affected by the NPU
// teardown hard-lock.
func usbDeviceUsesMT76(usbDev string) bool {
	devDir := filepath.Join("/sys/bus/usb/devices", usbDev)
	ifaces, _ := filepath.Glob(devDir + ":*")
	bound := false
	for _, i := range ifaces {
		link, err := os.Readlink(filepath.Join(i, "driver"))
		if err != nil {
			continue
		}
		bound = true
		if strings.HasPrefix(filepath.Base(link), "mt76") {
			return true
		}
	}
	if bound {
		return false // bound to a non-mt76 driver
	}
	// No driver bound (failed probe): fall back to the MediaTek USB vendor IDs.
	vendor := strings.ToLower(readSysAttr(filepath.Join(devDir, "idVendor")))
	return vendor == "0e8d" || vendor == "0489"
}
