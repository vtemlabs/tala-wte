// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

package iface

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// usbIDsPaths are the common locations of the system usb.ids database.
var usbIDsPaths = []string{"/usr/share/misc/usb.ids", "/usr/share/hwdata/usb.ids", "/var/lib/usbutils/usb.ids"}

// lookupUSBIDs returns the vendor/product names for a "vid:pid" from the system
// usb.ids database, the fallback for adapters not in wirelessDeviceDB. Returns
// empty strings if usb.ids is unavailable or the IDs are unlisted.
func lookupUSBIDs(usbID string) (vendor, product string) {
	parts := strings.SplitN(usbID, ":", 2)
	if len(parts) != 2 {
		return "", ""
	}
	vid, pid := strings.ToLower(parts[0]), strings.ToLower(parts[1])
	var f *os.File
	for _, p := range usbIDsPaths {
		if file, err := os.Open(p); err == nil {
			f = file
			break
		}
	}
	if f == nil {
		return "", ""
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	inVendor := false
	for sc.Scan() {
		line := sc.Text()
		if line == "" || line[0] == '#' {
			continue
		}
		if line[0] != '\t' { // vendor line: "vid  Vendor Name"
			if inVendor {
				break // moved past our vendor block without finding the product
			}
			if len(line) >= 6 && strings.ToLower(line[:4]) == vid {
				vendor = strings.TrimSpace(line[4:])
				inVendor = true
			}
			continue
		}
		if inVendor { // product line: "\tpid  Product Name"
			t := strings.TrimPrefix(line, "\t")
			if len(t) >= 6 && strings.ToLower(t[:4]) == pid {
				product = strings.TrimSpace(t[4:])
				break
			}
		}
	}
	return vendor, product
}

// WirelessVariant is an alternative product sharing the same USB ID and chipset
// as the canonical DeviceInfo entry; only the branding differs. OUIs lists the
// MAC prefixes that identify this variant, telling same-USB-ID cards apart.
type WirelessVariant struct {
	Manufacturer string   `json:"manufacturer"`
	Model        string   `json:"model"`
	OUIs         []string `json:"ouis,omitempty"`
}

// DeviceInfo holds the hardware capabilities and safety limits for a
// known wireless adapter, keyed by USB vendor:product ID.
type DeviceInfo struct {
	Manufacturer string   `json:"manufacturer"`
	Model        string   `json:"model"`
	Chipset      string   `json:"chipset"`
	Bands        []string `json:"bands"`
	// APBands are the bands this chip can host an AP on (hostapd beacon); can be
	// narrower than Bands. Empty means same as Bands.
	APBands           []string `json:"ap_bands,omitempty"`
	Standard          string   `json:"standard"`
	TxPowerAdjustable bool     `json:"tx_power_adjustable"`
	SafeMaxTxPower24  int      `json:"safe_max_tx_power_24"` // mBm, 0 = N/A
	SafeMaxTxPower5   int      `json:"safe_max_tx_power_5"`  // mBm, 0 = N/A
	HardCeiling       int      `json:"hard_ceiling"`         // mBm absolute max
	MonitorBands      []string `json:"monitor_bands"`
	InjectionBands    []string `json:"injection_bands"`
	MaxChannelWidth   int      `json:"max_channel_width"` // MHz
	HasDFS            bool     `json:"has_dfs"`
	Notes             string   `json:"notes,omitempty"`
	// Factory-shipped antenna values for RSSI calibration; users can override.
	StockAntennaGainDBI float64 `json:"stock_antenna_gain_dbi"` // dBi per antenna
	StockAntennaCount   int     `json:"stock_antenna_count"`
	AntennaConnector    string  `json:"antenna_connector"`
	// OUIs are the MAC prefixes identifying the canonical product, used to
	// disambiguate it from Variants. Empty = fall back to canonical branding.
	OUIs []string `json:"ouis,omitempty"`
	// Variants lists other physical products sharing this USB ID + chipset.
	Variants []WirelessVariant `json:"variants,omitempty"`
}

var wirelessDeviceDB = map[string]DeviceInfo{
	"0bda:8812": {
		Manufacturer: "ALFA Network", Model: "AWUS036ACH", Chipset: "RTL8812AU",
		Bands: []string{"2.4 GHz", "5 GHz"}, Standard: "802.11a/b/g/n/ac",
		TxPowerAdjustable: true, SafeMaxTxPower24: 3000, SafeMaxTxPower5: 3000, HardCeiling: 3150,
		MonitorBands: []string{"2.4 GHz", "5 GHz"}, InjectionBands: []string{"2.4 GHz", "5 GHz"},
		MaxChannelWidth: 80, HasDFS: true,
		Notes:               "Highest TX power (30 dBm). Out-of-tree driver (88XXau).",
		StockAntennaGainDBI: 5, StockAntennaCount: 2, AntennaConnector: "RP-SMA",
	},
	"0e8d:7612": {
		Manufacturer: "ALFA Network", Model: "AWUS036ACM", Chipset: "MT7612U",
		Bands: []string{"2.4 GHz", "5 GHz"}, Standard: "802.11a/b/g/n/ac",
		TxPowerAdjustable: true, SafeMaxTxPower24: 2300, SafeMaxTxPower5: 2000, HardCeiling: 2500,
		MonitorBands: []string{"2.4 GHz", "5 GHz"}, InjectionBands: []string{"2.4 GHz", "5 GHz"},
		MaxChannelWidth: 80, HasDFS: false,
		Notes:               "In-kernel mt76 driver. ~450ms channel switch latency. Injection limited to ~1000 pps.",
		StockAntennaGainDBI: 5, StockAntennaCount: 2, AntennaConnector: "RP-SMA",
		OUIs: []string{"00:c0:ca"}, // ALFA; a non-ALFA OUI resolves to the variant.
		Variants: []WirelessVariant{
			{Manufacturer: "Panda Wireless", Model: "PAU0D"},
		},
	},
	"0e8d:7961": {
		Manufacturer: "ALFA Network", Model: "AWUS036AXM", Chipset: "MT7921AU",
		Bands: []string{"2.4 GHz", "5 GHz", "6 GHz"},
		// 6 GHz is monitor/client only here; the driver won't beacon a 6 GHz AP.
		APBands:           []string{"2.4 GHz", "5 GHz"},
		Standard:          "802.11a/b/g/n/ac/ax (WiFi 6E)",
		TxPowerAdjustable: true, SafeMaxTxPower24: 1800, SafeMaxTxPower5: 1700, HardCeiling: 1900,
		MonitorBands: []string{"2.4 GHz", "5 GHz", "6 GHz"}, InjectionBands: []string{"2.4 GHz", "5 GHz"},
		MaxChannelWidth: 160, HasDFS: false,
		Notes:               "Lab-tested: 2.4 GHz inject 96-100%, 5 GHz inject 76-96%. 6 GHz monitor works (tri-band), but 6 GHz injection silently blocked by driver. 6 GHz requires US/GB/NZ reg domain. Use GB for best 6 GHz coverage (23 dBm).",
		StockAntennaGainDBI: 5, StockAntennaCount: 2, AntennaConnector: "RP-SMA",
		// ALFA's registered OUI; the Panda AXE3000 ships MediaTek's reference OUI,
		// so the MAC tells the two same-USB-ID adapters apart.
		OUIs: []string{"00:c0:ca"},
		Variants: []WirelessVariant{
			{Manufacturer: "Panda Wireless", Model: "AXE3000", OUIs: []string{"9c:ef:d5"}},
		},
	},
	"148f:5572": {
		Manufacturer: "Panda Wireless", Model: "PAU09", Chipset: "RT5572",
		Bands: []string{"2.4 GHz", "5 GHz"}, Standard: "802.11a/b/g/n",
		TxPowerAdjustable: false, SafeMaxTxPower24: 0, SafeMaxTxPower5: 0, HardCeiling: 2000,
		MonitorBands: []string{"2.4 GHz", "5 GHz"}, InjectionBands: []string{"2.4 GHz", "5 GHz"},
		MaxChannelWidth: 40, HasDFS: false,
		Notes:               "TX power always at max (driver ignores commands). No 80 MHz width. Reliable dual-band.",
		StockAntennaGainDBI: 5, StockAntennaCount: 2, AntennaConnector: "RP-SMA",
	},
	"148f:3572": {
		Manufacturer: "ALFA Network", Model: "AWUS051NH", Chipset: "RT3572",
		Bands: []string{"2.4 GHz", "5 GHz"}, Standard: "802.11a/b/g/n",
		TxPowerAdjustable: false, SafeMaxTxPower24: 0, SafeMaxTxPower5: 0, HardCeiling: 2000,
		MonitorBands: []string{"2.4 GHz", "5 GHz"}, InjectionBands: []string{"2.4 GHz"},
		MaxChannelWidth: 40, HasDFS: false,
		Notes:               "5 GHz injection BROKEN (documented bug). 1x2 MIMO (1 TX). TX power always at max.",
		StockAntennaGainDBI: 5, StockAntennaCount: 1, AntennaConnector: "RP-SMA",
	},
	"148f:3070": {
		Manufacturer: "ALFA Network", Model: "AWUS036NH", Chipset: "RT3070",
		Bands: []string{"2.4 GHz"}, Standard: "802.11b/g/n",
		TxPowerAdjustable: false, SafeMaxTxPower24: 0, SafeMaxTxPower5: 0, HardCeiling: 3000,
		MonitorBands: []string{"2.4 GHz"}, InjectionBands: []string{"2.4 GHz"},
		MaxChannelWidth: 40, HasDFS: false,
		Notes:               "2.4 GHz ONLY. High power (2W/30 dBm). TX always at max. No 5 GHz or 6 GHz support.",
		StockAntennaGainDBI: 5, StockAntennaCount: 1, AntennaConnector: "RP-SMA",
	},
}

// readMacAddress reads the hardware MAC address from sysfs as lowercase
// colon-separated hex, or empty string on failure.
func readMacAddress(ifaceName string) string {
	data, err := os.ReadFile(filepath.Join("/sys/class/net", ifaceName, "address"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// readUSBID reads the USB vendor and product IDs from sysfs and returns them as
// "vendor:product" (e.g. "0e8d:7612").
func readUSBID(ifaceName string) string {
	deviceLink := filepath.Join("/sys/class/net", ifaceName, "device")

	resolved, err := filepath.EvalSymlinks(deviceLink)
	if err != nil {
		log.Printf("[iface] readUSBID: %s EvalSymlinks failed: %v", ifaceName, err)
		return readUSBIDFromUevent(deviceLink)
	}

	// Walk up the USB sysfs tree (interface -> device) for idVendor/idProduct.
	dir := resolved
	for i := 0; i < 5; i++ {
		vendor := readSysfsTrimmed(filepath.Join(dir, "idVendor"))
		product := readSysfsTrimmed(filepath.Join(dir, "idProduct"))
		if vendor != "" && product != "" {
			log.Printf("[iface] readUSBID: %s -> %s:%s (found at %s)", ifaceName, vendor, product, dir)
			return vendor + ":" + product
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	result := readUSBIDFromUevent(deviceLink)
	if result != "" {
		log.Printf("[iface] readUSBID: %s -> %s (from uevent fallback)", ifaceName, result)
	}

	return result
}

// readUSBIDFromUevent parses the uevent file for PRODUCT=vendor/product/...
func readUSBIDFromUevent(deviceDir string) string {
	for _, rel := range []string{"uevent", "../uevent", "../../uevent"} {
		path := filepath.Join(deviceDir, rel)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PRODUCT=") {
				// PRODUCT=e8d/7612/100 -> "0e8d:7612"
				parts := strings.Split(strings.TrimPrefix(line, "PRODUCT="), "/")
				if len(parts) >= 2 {
					vendor := parts[0]
					product := parts[1]
					for len(vendor) < 4 {
						vendor = "0" + vendor
					}
					for len(product) < 4 {
						product = "0" + product
					}
					return vendor + ":" + product
				}
			}
		}
	}
	return ""
}

func readSysfsTrimmed(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// LookupDevice returns the DeviceInfo for a USB ID, or nil if unknown.
func LookupDevice(usbID string) *DeviceInfo {
	if info, ok := wirelessDeviceDB[usbID]; ok {
		return &info
	}
	return nil
}

// ouiVendorDB maps a 24-bit MAC OUI to its registered vendor, curated just enough
// to tell an ALFA card from a chip-vendor reference card with the same USB ID.
var ouiVendorDB = map[string]string{
	"00:c0:ca": "ALFA Network",
	"9c:ef:d5": "MediaTek",
	"00:0c:e7": "MediaTek",
	"00:e0:4c": "Realtek",
	"00:0c:43": "Ralink",
}

// ouiPrefix returns the lowercase 24-bit OUI ("00:c0:ca") of a MAC, or "".
func ouiPrefix(mac string) string {
	mac = strings.ToLower(strings.TrimSpace(mac))
	parts := strings.Split(mac, ":")
	if len(parts) < 3 || len(parts[0]) != 2 {
		return ""
	}
	return parts[0] + ":" + parts[1] + ":" + parts[2]
}

// OUIVendor returns the registered vendor for a MAC's OUI, or "" if unknown.
func OUIVendor(mac string) string {
	return ouiVendorDB[ouiPrefix(mac)]
}

func ouiListContains(list []string, oui string) bool {
	for _, o := range list {
		if strings.ToLower(o) == oui {
			return true
		}
	}
	return false
}

// brandsMatch reports whether an OUI vendor and a device manufacturer name the
// same brand (case-insensitive substring either way).
func brandsMatch(ouiVendor, manufacturer string) bool {
	a := strings.ToLower(strings.TrimSpace(ouiVendor))
	b := strings.ToLower(strings.TrimSpace(manufacturer))
	if a == "" || b == "" {
		return false
	}
	return strings.Contains(b, a) || strings.Contains(a, b)
}

// BrandForMAC disambiguates the manufacturer/model for an adapter whose USB ID is
// shared by several products by matching the MAC OUI against the canonical entry
// and its variants, returning the canonical branding when there is nothing to
// disambiguate or the OUI is unrecognized.
func (d *DeviceInfo) BrandForMAC(mac string) (manufacturer, model string) {
	manufacturer, model = d.Manufacturer, d.Model
	if len(d.Variants) == 0 {
		return
	}
	oui := ouiPrefix(mac)
	if oui == "" {
		return
	}
	// Explicit OUI match wins: canonical first, then each variant.
	if ouiListContains(d.OUIs, oui) {
		return d.Manufacturer, d.Model
	}
	for _, v := range d.Variants {
		if ouiListContains(v.OUIs, oui) {
			return v.Manufacturer, v.Model
		}
	}
	// Heuristic: an unlisted OUI whose vendor is clearly not the canonical brand,
	// with a single variant, is that variant.
	if v := ouiVendorDB[oui]; v != "" && !brandsMatch(v, d.Manufacturer) && len(d.Variants) == 1 {
		return d.Variants[0].Manufacturer, d.Variants[0].Model
	}
	return
}

// parseTxPowerFromIwDev extracts current TX power in mBm from `iw dev` info.
func parseTxPowerFromIwDev(iwOutput string) int {
	for _, line := range strings.Split(iwOutput, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "txpower ") {
			// "txpower 20.00 dBm"
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				if val, err := strconv.ParseFloat(parts[1], 64); err == nil {
					return int(val * 100)
				}
			}
		}
	}
	return 0
}

// parseMaxTxPowerFromIwPhy scans `iw phy info` output for the highest TX
// power across all frequency entries (lines like "* 2412 MHz [1] (20.0 dBm)").
func parseMaxTxPowerFromIwPhy(iwOutput string) int {
	maxMBm := 0
	for _, line := range strings.Split(iwOutput, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "* ") || !strings.Contains(trimmed, "MHz") {
			continue
		}
		// Extract the dBm value from "(20.0 dBm)".
		start := strings.LastIndex(trimmed, "(")
		end := strings.LastIndex(trimmed, " dBm)")
		if start < 0 || end < 0 || end <= start {
			continue
		}
		valStr := trimmed[start+1 : end]
		if val, err := strconv.ParseFloat(valStr, 64); err == nil {
			mBm := int(val * 100)
			if mBm > maxMBm {
				maxMBm = mBm
			}
		}
	}
	return maxMBm
}
