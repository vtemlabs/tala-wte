// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package iface

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Adapter represents a wireless network interface with its hardware and state.
type Adapter struct {
	Interface         string   `json:"interface"`
	Driver            string   `json:"driver"`
	Chipset           string   `json:"chipset"`
	Manufacturer      string   `json:"manufacturer"`
	DeviceModel       string   `json:"device_model"`
	USBID             string   `json:"usb_id"`
	CurrentMode       string   `json:"current_mode"`
	CurrentChannel    int      `json:"current_channel"`
	SupportedModes    []string `json:"supported_modes"`
	Bands             []string `json:"bands"`
	APBands           []string `json:"ap_bands,omitempty"` // subset of Bands the card can host an AP on
	Standard          string   `json:"standard"`
	Phy               string   `json:"phy"`
	TxPowerCurrent    int      `json:"tx_power_current"`
	TxPowerMax        int      `json:"tx_power_max"`
	TxPowerAdjustable bool     `json:"tx_power_adjustable"`
	MacAddress        string   `json:"mac_address"`
	MonitorBands      []string `json:"monitor_bands"`
	InjectionBands    []string `json:"injection_bands"`
	MaxChannelWidth   int      `json:"max_channel_width"`
	HasDFS            bool     `json:"has_dfs"`
	Notes             string   `json:"notes,omitempty"`
	StockAntennaGain  float64  `json:"stock_antenna_gain_dbi"`
	StockAntennaCount int      `json:"stock_antenna_count"`
	AntennaConnector  string   `json:"antenna_connector"`
	Limits            []string `json:"limits,omitempty"` // plain-language capability limits, computed in discovery
}

// virtualDrivers lists kernel drivers that produce simulated (non-RF) interfaces.
var virtualDrivers = []string{"mac80211_hwsim", "virt_wifi"}

// IsVirtualDriver returns true if the driver name is a known virtual/simulated driver.
func IsVirtualDriver(driver string) bool {
	for _, v := range virtualDrivers {
		if driver == v {
			return true
		}
	}
	return false
}

// computeLimits derives plain-language capability limits from a populated Adapter
// so the UI and logs can tell operators what a card cannot do. It only asserts a
// limit when the underlying capability is actually known (bands populated, or the
// device is in the curated table), so unknown adapters are never wrongly flagged.
func computeLimits(a Adapter) []string {
	var limits []string
	has := func(list []string, band string) bool {
		for _, b := range list {
			if strings.HasPrefix(b, band) {
				return true
			}
		}
		return false
	}

	// Band coverage (only when we actually know the bands).
	if len(a.Bands) > 0 && !has(a.Bands, "5") && !has(a.Bands, "6") {
		limits = append(limits, "2.4 GHz only (no 5/6 GHz)")
	}

	// AP host narrower than the radio bands (can tune a band but not beacon on it).
	if len(a.APBands) > 0 && len(a.APBands) < len(a.Bands) {
		if has(a.Bands, "5") && !has(a.APBands, "5") {
			limits = append(limits, "No 5 GHz AP (5 GHz is client/monitor only)")
		}
		if has(a.Bands, "6") && !has(a.APBands, "6") {
			limits = append(limits, "No 6 GHz AP (6 GHz is client/monitor only)")
		}
	}

	// WPA3-SAE: only assert for known chipsets that lack it (legacy needs PMF).
	if a.USBID != "" {
		if info := LookupDevice(a.USBID); info != nil && !info.SupportsWPA3SAE {
			limits = append(limits, "No WPA3-SAE (legacy chipset)")
		}
	}

	// Frame injection narrower than the radio bands (matters for security testing).
	if len(a.InjectionBands) > 0 && len(a.InjectionBands) < len(a.Bands) && has(a.Bands, "5") && !has(a.InjectionBands, "5") {
		limits = append(limits, "No 5 GHz frame injection")
	}

	// Channel width.
	if a.MaxChannelWidth > 0 && a.MaxChannelWidth < 80 {
		limits = append(limits, fmt.Sprintf("Max channel width %d MHz (no 80/160 MHz)", a.MaxChannelWidth))
	}

	return limits
}

// BestRealAdapter returns the first non-virtual adapter, or nil if none exist.
func BestRealAdapter(adapters []Adapter) *Adapter {
	for i := range adapters {
		if !IsVirtualDriver(adapters[i].Driver) {
			return &adapters[i]
		}
	}
	return nil
}

// FindByInterface returns the adapter matching the given interface name, or nil.
func FindByInterface(adapters []Adapter, ifName string) *Adapter {
	for i := range adapters {
		if adapters[i].Interface == ifName {
			return &adapters[i]
		}
	}
	return nil
}

// ResolveInterface picks the best interface to use for AP mode.
// Priority:
//  1. If the requested interface exists and is real hardware, use it.
//  2. If the requested interface is virtual, substitute the best real adapter.
//  3. If the requested interface doesn't exist, substitute the best real adapter.
//  4. If no real adapter exists, fall back to the requested interface (virtual).
func ResolveInterface(adapters []Adapter, requested string) (resolved string, substituted bool, reason string) {
	req := FindByInterface(adapters, requested)

	if req != nil && !IsVirtualDriver(req.Driver) {
		return requested, false, ""
	}

	// Requested is virtual or missing; look for real hardware.
	real := BestRealAdapter(adapters)
	if real != nil {
		if req == nil {
			return real.Interface, true, fmt.Sprintf("configured interface %q not found, using %s (%s)", requested, real.Interface, real.Driver)
		}
		return real.Interface, true, fmt.Sprintf("configured interface %q is virtual (%s), using real hardware %s (%s)", requested, req.Driver, real.Interface, real.Driver)
	}

	if req != nil {
		return requested, false, fmt.Sprintf("no real wireless hardware detected, using virtual adapter %s (%s) - will not broadcast RF", requested, req.Driver)
	}
	return requested, false, fmt.Sprintf("configured interface %q not found and no alternatives available", requested)
}

// BestFreeRealAdapter returns the first real adapter not in the inUse set, or nil.
func BestFreeRealAdapter(adapters []Adapter, inUse map[string]bool) *Adapter {
	for i := range adapters {
		if !IsVirtualDriver(adapters[i].Driver) && !inUse[adapters[i].Interface] {
			return &adapters[i]
		}
	}
	return nil
}

// ResolveInterfaceFree is the allocation-aware resolver: it never returns an
// interface already in use by a running network, auto-substituting a free real
// adapter and erroring when none is free. inUse guards the start-race window
// before a running network's PHY has moved into its namespace.
func ResolveInterfaceFree(adapters []Adapter, requested string, inUse map[string]bool) (resolved string, substituted bool, reason string, err error) {
	req := FindByInterface(adapters, requested)

	if req != nil && !IsVirtualDriver(req.Driver) && !inUse[requested] {
		return requested, false, "", nil
	}

	if free := BestFreeRealAdapter(adapters, inUse); free != nil {
		switch {
		case inUse[requested]:
			reason = fmt.Sprintf("interface %q is in use by another running network, using free adapter %s (%s)", requested, free.Interface, free.Driver)
		case req != nil:
			reason = fmt.Sprintf("interface %q is virtual (%s), using real adapter %s (%s)", requested, req.Driver, free.Interface, free.Driver)
		default:
			reason = fmt.Sprintf("interface %q not found, using free adapter %s (%s)", requested, free.Interface, free.Driver)
		}
		return free.Interface, true, reason, nil
	}

	if inUse[requested] {
		return "", false, "", fmt.Errorf("interface %q is already in use by another running network and no other wireless adapter is free", requested)
	}
	return "", false, "", fmt.Errorf("no free wireless adapter available (configured %q is not a free real adapter)", requested)
}

// DiscoverAdapters enumerates all wireless interfaces and enriches them.
func DiscoverAdapters() []Adapter {
	ifaces := wirelessInterfaces()
	adapters := make([]Adapter, 0, len(ifaces))

	for _, ifName := range ifaces {
		a := Adapter{Interface: ifName}

		driverLink := filepath.Join("/sys/class/net", ifName, "device", "driver")
		if target, err := os.Readlink(driverLink); err == nil {
			a.Driver = filepath.Base(target)
		}

		a.MacAddress = readMacAddress(ifName)

		a.USBID = readUSBID(ifName)
		if a.USBID != "" {
			if info := LookupDevice(a.USBID); info != nil {
				a.Chipset = info.Chipset
				// The MAC OUI disambiguates adapters sharing a USB ID.
				a.Manufacturer, a.DeviceModel = info.BrandForMAC(a.MacAddress)
				a.Bands = info.Bands
				a.APBands = info.APBands
				a.Standard = info.Standard
				a.TxPowerAdjustable = info.TxPowerAdjustable
				a.MonitorBands = info.MonitorBands
				a.InjectionBands = info.InjectionBands
				a.MaxChannelWidth = info.MaxChannelWidth
				a.HasDFS = info.HasDFS
				a.Notes = info.Notes
				a.StockAntennaGain = info.StockAntennaGainDBI
				a.StockAntennaCount = info.StockAntennaCount
				a.AntennaConnector = info.AntennaConnector
			}
		}

		// Fall back to the system usb.ids for adapters not in the curated DB.
		if a.Manufacturer == "" && a.USBID != "" {
			if v, p := lookupUSBIDs(a.USBID); v != "" || p != "" {
				a.Manufacturer = v
				a.DeviceModel = p
			}
		}

		// Last resort: derive the vendor from the MAC OUI.
		if a.Manufacturer == "" {
			if v := OUIVendor(a.MacAddress); v != "" {
				a.Manufacturer = v
			}
		}

		if a.Chipset == "" {
			ueventPath := filepath.Join("/sys/class/net", ifName, "device", "uevent")
			if data, err := os.ReadFile(ueventPath); err == nil {
				for _, line := range strings.Split(string(data), "\n") {
					if strings.HasPrefix(line, "DRIVER=") {
						a.Chipset = strings.TrimPrefix(line, "DRIVER=")
						break
					}
				}
			}
		}

		mode, channel, phyName, _ := parseIwDevInfo(ifName)
		a.CurrentMode = mode
		a.CurrentChannel = channel
		a.Phy = phyName

		if iwOut, err := execCapture("iw", "dev", ifName, "info"); err == nil {
			a.TxPowerCurrent = parseTxPowerFromIwDev(iwOut)
		}

		if phyName != "" {
			if phyOut, err := execCapture("iw", "phy", phyName, "info"); err == nil {
				a.SupportedModes = parseIwPhySupportedModesFromOutput(phyOut)
				a.TxPowerMax = parseMaxTxPowerFromIwPhy(phyOut)
			}
		}

		a.Limits = computeLimits(a)
		adapters = append(adapters, a)
	}

	// Deduplicate monitor wrappers if on the same PHY
	if len(adapters) > 1 {
		phyMap := make(map[string][]int)
		for i, a := range adapters {
			if a.Phy != "" {
				phyMap[a.Phy] = append(phyMap[a.Phy], i)
			}
		}

		drop := make(map[int]bool)
		for _, indices := range phyMap {
			if len(indices) <= 1 {
				continue
			}
			primaryIdx := -1
			for _, idx := range indices {
				if !strings.HasSuffix(adapters[idx].Interface, "mon") {
					primaryIdx = idx
					break
				}
			}
			if primaryIdx < 0 {
				primaryIdx = indices[0]
			}

			for _, idx := range indices {
				if idx != primaryIdx {
					drop[idx] = true
				}
			}
		}

		if len(drop) > 0 {
			deduped := make([]Adapter, 0, len(adapters)-len(drop))
			for i, a := range adapters {
				if !drop[i] {
					deduped = append(deduped, a)
				}
			}
			adapters = deduped
		}
	}

	return adapters
}

func wirelessInterfaces() []string {
	var names []string
	f, err := os.Open("/proc/net/wireless")
	if err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			if lineNo <= 2 {
				continue
			}
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, ":", 2)
			if len(parts) >= 1 {
				names = append(names, strings.TrimSpace(parts[0]))
			}
		}
	}

	// Sysfs fallback
	entries, err := os.ReadDir("/sys/class/net")
	if err == nil {
		for _, entry := range entries {
			ifName := entry.Name()
			wirelessDir := filepath.Join("/sys/class/net", ifName, "wireless")
			if fi, err := os.Stat(wirelessDir); err == nil && fi.IsDir() {
				found := false
				for _, n := range names {
					if n == ifName {
						found = true
						break
					}
				}
				if !found {
					names = append(names, ifName)
				}
			}
		}
	}
	return names
}

func parseIwDevInfo(iface string) (mode string, channel int, phyName string, err error) {
	out, err := execCapture("iw", "dev", iface, "info")
	if err != nil {
		return "", 0, "", err
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "type ") {
			mode = strings.TrimPrefix(line, "type ")
		}
		if strings.HasPrefix(line, "channel ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				channel, _ = strconv.Atoi(fields[1])
			}
		}
		if strings.HasPrefix(line, "wiphy ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				phyName = "phy" + fields[1]
			}
		}
	}
	return mode, channel, phyName, nil
}

func parseIwPhySupportedModesFromOutput(out string) []string {
	var modes []string
	inBlock := false
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "Supported interface modes:" {
			inBlock = true
			continue
		}
		if inBlock {
			if strings.HasPrefix(trimmed, "* ") {
				modes = append(modes, strings.TrimPrefix(trimmed, "* "))
			} else {
				break
			}
		}
	}
	return modes
}

// execCapture runs a command with a hard timeout. A wedged radio makes iw block
// indefinitely in the kernel; the timeout lets the scan fail and move on instead
// of hanging the caller forever.
func execCapture(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	// WaitDelay force-closes the I/O pipes shortly after the context kills the
	// process, so Wait cannot hang if a stuck child inherited the pipe - the
	// timeout is then a hard bound regardless of how the command wedged.
	cmd.WaitDelay = 3 * time.Second
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("command %s failed: %w, stderr: %s", name, err, stderr.String())
	}
	return stdout.String(), nil
}
