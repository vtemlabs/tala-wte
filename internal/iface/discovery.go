// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package iface

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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

func execCapture(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("command %s failed: %w, stderr: %s", name, err, stderr.String())
	}
	return stdout.String(), nil
}
