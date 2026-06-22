// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

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
	SupportsWPA3SAE   bool     `json:"supports_wpa3_sae"` // WPA3-SAE needs PMF (802.11w); false on legacy chipsets
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
		SupportsWPA3SAE: true,
		Bands:           []string{"2.4 GHz", "5 GHz"}, Standard: "802.11a/b/g/n/ac",
		TxPowerAdjustable: true, SafeMaxTxPower24: 3000, SafeMaxTxPower5: 3000, HardCeiling: 3150,
		MonitorBands: []string{"2.4 GHz", "5 GHz"}, InjectionBands: []string{"2.4 GHz", "5 GHz"},
		MaxChannelWidth: 80, HasDFS: true,
		Notes:               "Highest TX power (30 dBm). Out-of-tree driver (88XXau).",
		StockAntennaGainDBI: 5, StockAntennaCount: 2, AntennaConnector: "RP-SMA",
	},
	"0e8d:7612": {
		Manufacturer: "ALFA Network", Model: "AWUS036ACM", Chipset: "MT7612U",
		SupportsWPA3SAE: true,
		Bands:           []string{"2.4 GHz", "5 GHz"}, Standard: "802.11a/b/g/n/ac",
		TxPowerAdjustable: true, SafeMaxTxPower24: 2300, SafeMaxTxPower5: 2000, HardCeiling: 2500,
		MonitorBands: []string{"2.4 GHz", "5 GHz"}, InjectionBands: []string{"2.4 GHz", "5 GHz"},
		MaxChannelWidth: 80, HasDFS: false,
		Notes:               "In-kernel mt76 driver. ~450ms channel switch latency. Injection limited to ~1000 pps.",
		StockAntennaGainDBI: 5, StockAntennaCount: 2, AntennaConnector: "RP-SMA",
		// 0e8d:7612 is MediaTek's generic MT7612U USB ID, shared by the ALFA
		// AWUS036ACM and various clones that ship with a MediaTek reference MAC, so
		// the OUI cannot reliably tell them apart. Report the dominant real product
		// (the ALFA AWUS036ACM) rather than guess an unverifiable clone SKU.
	},
	"0e8d:7961": {
		Manufacturer: "ALFA Network", Model: "AWUS036AXM", Chipset: "MT7921AU",
		SupportsWPA3SAE: true,
		Bands:           []string{"2.4 GHz", "5 GHz", "6 GHz"},
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

// OnboardWiFi maps a board identifier to the curated capabilities of that
// board's soldered (SDIO/PCIe) Wi-Fi. Onboard radios on single-board computers
// and mini-PCs carry no USB ID, so they are recognized by the board model
// (device-tree on ARM, DMI on x86) instead of a USB vendor:product key.
type OnboardWiFi struct {
	// Match holds case-insensitive substrings tested against the board model;
	// the first matching entry wins, so order entries most-specific first
	// (e.g. "Pi 3 Model B Plus" before "Pi 3 Model B").
	Match []string
	Info  DeviceInfo
}

// onboardWiFiDB lists the soldered Wi-Fi found on common SBCs/mini-PCs. The
// chip determines the RF capability; the board model gives the friendly name.
// Onboard radios generally do not do monitor/injection and run low TX power on
// a single PCB antenna - which is fine for Tala WTE's AP-hosting role.
var onboardWiFiDB = []OnboardWiFi{
	// --- Raspberry Pi 400: BCM43456 dual-band 802.11ac (checked before the
	// generic dual-band entry; same RF class as 43455 but different firmware) ---
	{
		Match: []string{"Raspberry Pi 400"},
		Info: DeviceInfo{
			Manufacturer: "Raspberry Pi", Model: "Pi 400 onboard Wi-Fi (BCM43456)", Chipset: "BCM43456",
			Bands: []string{"2.4 GHz", "5 GHz"}, Standard: "802.11a/b/g/n/ac",
			SupportsWPA3SAE:   true,
			TxPowerAdjustable: false,
			MonitorBands:      []string{}, InjectionBands: []string{},
			MaxChannelWidth: 80, HasDFS: false,
			Notes:               "Onboard SDIO Wi-Fi (brcmfmac), 1x1 SISO. BCM43456 (sister of 43455, uses brcmfmac43456 firmware). Dual-band 802.11ac; 5 GHz AP limited to non-DFS UNII-1 (ch 36/40/44/48), HT40 on 2.4 GHz. set hostapd country_code for 5 GHz. No monitor/injection. WPA3-SAE AP is version-gated/flaky on brcmfmac; WPA2 is reliable.",
			StockAntennaGainDBI: 2, StockAntennaCount: 1, AntennaConnector: "Onboard (PCB)",
		},
	},
	// --- Raspberry Pi: BCM43455 / CYW43455 dual-band 802.11ac
	// (3B+, 3A+, 4, 5, CM4, CM5). Checked before the 2.4-only "3 Model B" entry. ---
	{
		Match: []string{
			"Raspberry Pi 4 Model B",
			"Raspberry Pi 5",
			"Raspberry Pi Compute Module 4", "Raspberry Pi Compute Module 5",
			"Raspberry Pi 3 Model B Plus", "Raspberry Pi 3 Model B+",
			"Raspberry Pi 3 Model A Plus", "Raspberry Pi 3 Model A+",
		},
		Info: DeviceInfo{
			Manufacturer: "Raspberry Pi", Model: "Onboard Wi-Fi (BCM43455)", Chipset: "BCM43455",
			Bands: []string{"2.4 GHz", "5 GHz"}, Standard: "802.11a/b/g/n/ac",
			SupportsWPA3SAE:   true,
			TxPowerAdjustable: false,
			MonitorBands:      []string{}, InjectionBands: []string{},
			MaxChannelWidth: 80, HasDFS: false,
			Notes:               "Onboard SDIO Wi-Fi (brcmfmac), 1x1 SISO, non-simultaneous dual-band. BCM43455/CYW43455 (Pi 5 / CM5 use the CYW43455 die). 5 GHz AP limited to non-DFS UNII-1 channels (36/40/44/48); HT40 on 2.4 GHz. Set hostapd country_code or 5 GHz is unusable. CM4/CM5 can route to a u.FL antenna. Low TX power, no monitor/injection. WPA3-SAE AP is version-gated/flaky on brcmfmac; WPA2 is reliable.",
			StockAntennaGainDBI: 2, StockAntennaCount: 1, AntennaConnector: "Onboard (PCB)",
		},
	},
	// --- Raspberry Pi Zero 2 W: BCM43436 2.4 GHz 802.11n ---
	{
		Match: []string{"Raspberry Pi Zero 2"},
		Info: DeviceInfo{
			Manufacturer: "Raspberry Pi", Model: "Zero 2 W onboard Wi-Fi (BCM43436)", Chipset: "BCM43436",
			Bands: []string{"2.4 GHz"}, Standard: "802.11b/g/n",
			SupportsWPA3SAE:   false,
			TxPowerAdjustable: false,
			MonitorBands:      []string{}, InjectionBands: []string{},
			MaxChannelWidth: 20, HasDFS: false,
			Notes:               "Onboard SDIO Wi-Fi (brcmfmac43436), 1x1 SISO. 2.4 GHz ONLY, 802.11n (20 MHz, ~150 Mbps PHY). WPA2 AP via hostapd. Chip die varies by board revision (43436/43430). No 5 GHz, WPA3-SAE, monitor, or injection.",
			StockAntennaGainDBI: 2, StockAntennaCount: 1, AntennaConnector: "Onboard (PCB)",
		},
	},
	// --- Raspberry Pi: BCM43438 2.4 GHz 802.11n (3B non-plus, Zero W) ---
	{
		Match: []string{
			"Raspberry Pi 3 Model B", // matches non-plus 3B (the "Plus" entry above wins for 3B+)
			"Raspberry Pi Zero W",
		},
		Info: DeviceInfo{
			Manufacturer: "Raspberry Pi", Model: "Onboard Wi-Fi (BCM43438)", Chipset: "BCM43438",
			Bands: []string{"2.4 GHz"}, Standard: "802.11b/g/n",
			SupportsWPA3SAE:   false,
			TxPowerAdjustable: false,
			MonitorBands:      []string{}, InjectionBands: []string{},
			MaxChannelWidth: 20, HasDFS: false,
			Notes:               "Onboard SDIO Wi-Fi (brcmfmac), 1x1 SISO. 2.4 GHz ONLY, 802.11n (20 MHz). WPA2 AP via hostapd. Single onboard antenna, low TX power. No 5 GHz, WPA3-SAE, monitor, or injection.",
			StockAntennaGainDBI: 2, StockAntennaCount: 1, AntennaConnector: "Onboard (PCB)",
		},
	},

	// ===== LattePanda (x86; onboard/CNVi radio is effectively board-fixed) =====
	// Most-specific model strings first ("3 Delta"/"Sigma" before bare "Delta"/"".)
	{
		Match: []string{"LattePanda 3 Delta", "LattePanda Sigma"},
		Info: DeviceInfo{
			Manufacturer: "LattePanda", Model: "Onboard Wi-Fi (Intel AX201)", Chipset: "Intel AX201",
			Bands: []string{"2.4 GHz", "5 GHz"}, APBands: []string{"2.4 GHz"}, Standard: "802.11a/b/g/n/ac/ax (Wi-Fi 6)",
			SupportsWPA3SAE: true, TxPowerAdjustable: false,
			MonitorBands: []string{}, InjectionBands: []string{},
			MaxChannelWidth: 160, HasDFS: false,
			Notes:               "Intel CNVi (iwlwifi). IMPORTANT: Intel cards can host an AP on 2.4 GHz ONLY - no 5/6 GHz AP on Linux (regulatory NO-IR). Great client; for 5 GHz AP hosting use an external MediaTek/Atheros USB adapter.",
			StockAntennaGainDBI: 2, StockAntennaCount: 2, AntennaConnector: "Onboard",
		},
	},
	{
		Match: []string{"LattePanda Alpha", "LattePanda Delta"},
		Info: DeviceInfo{
			Manufacturer: "LattePanda", Model: "Onboard Wi-Fi (Intel AC 7265)", Chipset: "Intel AC 7265",
			Bands: []string{"2.4 GHz", "5 GHz"}, APBands: []string{"2.4 GHz"}, Standard: "802.11a/b/g/n/ac",
			SupportsWPA3SAE: false, TxPowerAdjustable: false,
			MonitorBands: []string{}, InjectionBands: []string{},
			MaxChannelWidth: 80, HasDFS: false,
			Notes:               "Intel AC 7265 on a pre-installed M.2 A+E card (user-swappable). 2.4 GHz AP ONLY (no 5 GHz AP on Linux). Use an external MediaTek/Atheros USB adapter for 5 GHz AP hosting.",
			StockAntennaGainDBI: 2, StockAntennaCount: 2, AntennaConnector: "M.2 (A+E)",
		},
	},
	{
		Match: []string{"LattePanda"}, // V1 fallback (checked after the specific models above)
		Info: DeviceInfo{
			Manufacturer: "LattePanda", Model: "V1 onboard Wi-Fi (RTL8723BS)", Chipset: "RTL8723BS",
			Bands: []string{"2.4 GHz"}, Standard: "802.11b/g/n",
			SupportsWPA3SAE: false, TxPowerAdjustable: false,
			MonitorBands: []string{}, InjectionBands: []string{},
			MaxChannelWidth: 20, HasDFS: false,
			Notes:               "Soldered SDIO RTL8723BS (rtl8723bs). 2.4 GHz only, 802.11n; AP works but is flaky. No 5 GHz/WPA3/monitor.",
			StockAntennaGainDBI: 1, StockAntennaCount: 1, AntennaConnector: "Onboard",
		},
	},

	// ===== Khadas (ARM, soldered AP6xxx over SDIO/PCIe, brcmfmac) =====
	{
		Match: []string{"Khadas VIM4", "Khadas Edge2"},
		Info: DeviceInfo{
			Manufacturer: "Khadas", Model: "Onboard Wi-Fi (AP6275S / BCM43752)", Chipset: "BCM43752",
			Bands: []string{"2.4 GHz", "5 GHz"}, Standard: "802.11a/b/g/n/ac/ax (Wi-Fi 6)",
			SupportsWPA3SAE: true, TxPowerAdjustable: false,
			MonitorBands: []string{}, InjectionBands: []string{},
			MaxChannelWidth: 80, HasDFS: false,
			Notes:               "Soldered Ampak AP6275S (BCM43752, brcmfmac). Wi-Fi 6 on 2.4/5 GHz (no 6 GHz). AP works with brcmfmac FullMAC limits; 5 GHz AP non-DFS.",
			StockAntennaGainDBI: 2, StockAntennaCount: 2, AntennaConnector: "u.FL",
		},
	},
	{
		Match: []string{"Khadas VIM2", "Khadas VIM3", "Khadas Edge"},
		Info: DeviceInfo{
			Manufacturer: "Khadas", Model: "Onboard Wi-Fi (AP6398S / BCM4359)", Chipset: "BCM4359",
			Bands: []string{"2.4 GHz", "5 GHz"}, Standard: "802.11a/b/g/n/ac",
			SupportsWPA3SAE: false, TxPowerAdjustable: false,
			MonitorBands: []string{}, InjectionBands: []string{},
			MaxChannelWidth: 80, HasDFS: false,
			Notes:               "Soldered Ampak AP6398S (BCM4359, 2x2, brcmfmac). Dual-band 802.11ac. AP works with FullMAC limits; WPA3-SAE AP unreliable on this BCM generation; 5 GHz AP non-DFS.",
			StockAntennaGainDBI: 2, StockAntennaCount: 2, AntennaConnector: "u.FL",
		},
	},
	{
		Match: []string{"Khadas VIM1"},
		Info: DeviceInfo{
			Manufacturer: "Khadas", Model: "VIM1 onboard Wi-Fi (AP6212/AP6255)", Chipset: "BCM43438/BCM43455",
			Bands: []string{"2.4 GHz", "5 GHz"}, Standard: "802.11a/b/g/n/ac",
			SupportsWPA3SAE: false, TxPowerAdjustable: false,
			MonitorBands: []string{}, InjectionBands: []string{},
			MaxChannelWidth: 80, HasDFS: false,
			Notes:               "Soldered Ampak (brcmfmac): VIM1 Basic = AP6212 (BCM43438, 2.4 GHz n); VIM1 Pro = AP6255 (BCM43455, dual-band ac). AP works with FullMAC limits.",
			StockAntennaGainDBI: 2, StockAntennaCount: 1, AntennaConnector: "u.FL",
		},
	},

	// ===== Orange Pi (Broadcom AP6xxx = in-tree brcmfmac, reliable). NOTE: some
	// OrangePi model strings are shared across hardware revisions with different
	// silicon, so the chip can vary; entries note this where it applies. =====
	{
		// In-tree Broadcom dual-band Wi-Fi 5 boards (AP6255/AP6256/AP6356S).
		Match: []string{"OrangePi 4", "OrangePi Lite2", "Orange Pi RK3399", "OPi 5 Pro", "OPi CM5"},
		Info: DeviceInfo{
			Manufacturer: "Orange Pi", Model: "Onboard Wi-Fi (Broadcom AP6xxx)", Chipset: "BCM43456 (AP6256)",
			Bands: []string{"2.4 GHz", "5 GHz"}, Standard: "802.11a/b/g/n/ac",
			SupportsWPA3SAE: true, TxPowerAdjustable: false,
			MonitorBands: []string{}, InjectionBands: []string{},
			MaxChannelWidth: 80, HasDFS: false,
			Notes:               "Soldered Ampak AP6xxx (AP6255/56/356S; BCM4345x/4356, brcmfmac, in-tree). Dual-band 802.11ac, reliable AP; 5 GHz AP non-DFS. Exact AP6xxx part varies by board.",
			StockAntennaGainDBI: 2, StockAntennaCount: 2, AntennaConnector: "u.FL",
		},
	},
	{
		Match: []string{"OPi 5 Max"}, // AP6611 Wi-Fi 6E
		Info: DeviceInfo{
			Manufacturer: "Orange Pi", Model: "5 Max onboard Wi-Fi (AP6611)", Chipset: "BCM43xx (AP6611)",
			Bands: []string{"2.4 GHz", "5 GHz", "6 GHz"}, APBands: []string{"2.4 GHz", "5 GHz"}, Standard: "802.11a/b/g/n/ac/ax (Wi-Fi 6E)",
			SupportsWPA3SAE: true, TxPowerAdjustable: false,
			MonitorBands: []string{}, InjectionBands: []string{},
			MaxChannelWidth: 80, HasDFS: false,
			Notes:               "Soldered Ampak AP6611 (brcmfmac). Wi-Fi 6E silicon; 6 GHz operation is regdomain/driver-dependent (treat as uncertain). 5 GHz AP non-DFS, FullMAC limits.",
			StockAntennaGainDBI: 2, StockAntennaCount: 2, AntennaConnector: "u.FL",
		},
	},
	{
		Match: []string{"OrangePi Win"}, // AP6212, 2.4 GHz only
		Info: DeviceInfo{
			Manufacturer: "Orange Pi", Model: "Win onboard Wi-Fi (AP6212)", Chipset: "BCM43430 (AP6212)",
			Bands: []string{"2.4 GHz"}, Standard: "802.11b/g/n",
			SupportsWPA3SAE: false, TxPowerAdjustable: false,
			MonitorBands: []string{}, InjectionBands: []string{},
			MaxChannelWidth: 20, HasDFS: false,
			Notes:               "Soldered Ampak AP6212 (BCM43430, brcmfmac). 2.4 GHz only, 802.11n. WPA2 AP.",
			StockAntennaGainDBI: 2, StockAntennaCount: 1, AntennaConnector: "u.FL",
		},
	},
	{
		// UNISOC UWE5622 (brand AW859A/20U5622): out-of-tree driver, dual-band ac,
		// but AP/long-uptime are flaky. Zero2/Zero2 W/Zero3 listed before bare Zero.
		Match: []string{"OrangePi Zero2 W", "OrangePi Zero2", "OrangePi Zero3", "OrangePi 4 LTS", "Orange Pi CM4"},
		Info: DeviceInfo{
			Manufacturer: "Orange Pi", Model: "Onboard Wi-Fi (UNISOC UWE5622)", Chipset: "UNISOC UWE5622 (AW859A)",
			Bands: []string{"2.4 GHz", "5 GHz"}, Standard: "802.11a/b/g/n/ac",
			SupportsWPA3SAE: false, TxPowerAdjustable: false,
			MonitorBands: []string{}, InjectionBands: []string{},
			MaxChannelWidth: 80, HasDFS: false,
			Notes:               "UNISOC UWE5622 (AW859A/20U5622), OUT-OF-TREE uwe5622 driver. Dual-band Wi-Fi 5, but AP mode is marginal and connections can drop over uptime / with 802.11r. Prefer an external USB adapter for reliable AP hosting.",
			StockAntennaGainDBI: 2, StockAntennaCount: 1, AntennaConnector: "u.FL",
		},
	},
	{
		// Realtek RTL8189FTV 2.4 GHz (sunxi boards). "Lite2" handled above (Broadcom).
		Match: []string{"OrangePi PC Plus", "OrangePi Plus 2E", "OrangePi Lite"},
		Info: DeviceInfo{
			Manufacturer: "Orange Pi", Model: "Onboard Wi-Fi (RTL8189FTV)", Chipset: "RTL8189FTV",
			Bands: []string{"2.4 GHz"}, Standard: "802.11b/g/n",
			SupportsWPA3SAE: false, TxPowerAdjustable: false,
			MonitorBands: []string{}, InjectionBands: []string{},
			MaxChannelWidth: 20, HasDFS: false,
			Notes:               "Soldered Realtek RTL8189FTV (SDIO), out-of-tree rtl8189fs driver. 2.4 GHz only, 802.11n. SoftAP works.",
			StockAntennaGainDBI: 2, StockAntennaCount: 1, AntennaConnector: "u.FL",
		},
	},
	{
		Match: []string{"OrangePi Zero"}, // bare Zero / Zero LTS = Allwinner XR819 (after Zero2/3 above)
		Info: DeviceInfo{
			Manufacturer: "Orange Pi", Model: "Zero onboard Wi-Fi (XR819)", Chipset: "Allwinner XR819",
			Bands: []string{"2.4 GHz"}, Standard: "802.11n",
			SupportsWPA3SAE: false, TxPowerAdjustable: false,
			MonitorBands: []string{}, InjectionBands: []string{},
			MaxChannelWidth: 20, HasDFS: false,
			Notes:               "Allwinner XR819 (SDIO), out-of-tree xradio driver. 2.4 GHz only, 1x1. Notoriously buggy firmware; AP is poor. Use an external USB adapter.",
			StockAntennaGainDBI: 1, StockAntennaCount: 1, AntennaConnector: "u.FL",
		},
	},

	// ===== Radxa (ROCK Pi / ROCK). Many ROCK boards ship Wi-Fi only as an M.2
	// card (no onboard) - those are not listed here. Note "ROCK 4C+" drops "Pi". =====
	{
		Match: []string{"Radxa ROCK 5B+"}, // RTL8852BE, soldered (5B w/o "+" has no onboard)
		Info: DeviceInfo{
			Manufacturer: "Radxa", Model: "ROCK 5B+ onboard Wi-Fi (RTL8852BE)", Chipset: "RTL8852BE",
			Bands: []string{"2.4 GHz", "5 GHz"}, Standard: "802.11a/b/g/n/ac/ax (Wi-Fi 6)",
			SupportsWPA3SAE: true, TxPowerAdjustable: false,
			MonitorBands: []string{}, InjectionBands: []string{},
			MaxChannelWidth: 80, HasDFS: true,
			Notes:               "Soldered Realtek RTL8852BE (rtw89, in-tree), 2x2 Wi-Fi 6. One of the better onboard AP radios in the SBC set: native AP, DFS, WPA3-SAE.",
			StockAntennaGainDBI: 2, StockAntennaCount: 2, AntennaConnector: "u.FL",
		},
	},
	{
		// Broadcom AP6256/AW-CM256 dual-band 802.11ac (brcmfmac, in-tree).
		Match: []string{"Radxa ROCK Pi 4B", "Radxa ROCK 4C+", "Radxa ROCK Pi 4 SE", "Radxa ROCK 3A", "Radxa Zero2", "Radxa CM3"},
		Info: DeviceInfo{
			Manufacturer: "Radxa", Model: "Onboard Wi-Fi (Broadcom AP6256)", Chipset: "BCM43456 (AP6256)",
			Bands: []string{"2.4 GHz", "5 GHz"}, Standard: "802.11a/b/g/n/ac",
			SupportsWPA3SAE: true, TxPowerAdjustable: false,
			MonitorBands: []string{}, InjectionBands: []string{},
			MaxChannelWidth: 80, HasDFS: false,
			Notes:               "Soldered Ampak AP6256 / AW-CM256 (BCM43456/CYW43455, brcmfmac), 1x1 dual-band ac. Reliable AP via hostapd (FullMAC limits); 5 GHz AP non-DFS. Some SKUs have no onboard radio (probe at runtime).",
			StockAntennaGainDBI: 2, StockAntennaCount: 1, AntennaConnector: "u.FL",
		},
	},
	{
		Match: []string{"Radxa ROCK Pi S"}, // RTL8723DS, 2.4 GHz only (optional SKU)
		Info: DeviceInfo{
			Manufacturer: "Radxa", Model: "ROCK Pi S onboard Wi-Fi (RTL8723DS)", Chipset: "RTL8723DS",
			Bands: []string{"2.4 GHz"}, Standard: "802.11b/g/n",
			SupportsWPA3SAE: false, TxPowerAdjustable: false,
			MonitorBands: []string{}, InjectionBands: []string{},
			MaxChannelWidth: 20, HasDFS: false,
			Notes:               "Soldered Realtek RTL8723DS (SDIO), out-of-tree driver. 2.4 GHz only, 802.11n. Limited AP. Present only on the Wi-Fi SKU.",
			StockAntennaGainDBI: 1, StockAntennaCount: 1, AntennaConnector: "u.FL",
		},
	},

	// ===== Banana Pi. Note odd model strings: "Sinovoip BPI-M2", "BananaPi-M64"
	// (no spaces), "Banana Pi M2 Berry" (no "BPI-"), lowercase "Bananapi BPI-R3". =====
	{
		Match: []string{"Bananapi BPI-R3"}, // MT7986, native mt76 AP, excellent
		Info: DeviceInfo{
			Manufacturer: "Banana Pi", Model: "BPI-R3 onboard Wi-Fi (MT7986)", Chipset: "MediaTek MT7986",
			Bands: []string{"2.4 GHz", "5 GHz"}, Standard: "802.11a/b/g/n/ac/ax (Wi-Fi 6)",
			SupportsWPA3SAE: true, TxPowerAdjustable: false,
			MonitorBands: []string{"2.4 GHz", "5 GHz"}, InjectionBands: []string{"2.4 GHz", "5 GHz"},
			MaxChannelWidth: 160, HasDFS: true,
			Notes:               "Soldered MediaTek MT7986 (mt76, in-tree), 4x4 DBDC Wi-Fi 6. Excellent native AP: DFS, WPA3-SAE, monitor/injection, 160 MHz. Router-class radio.",
			StockAntennaGainDBI: 2, StockAntennaCount: 4, AntennaConnector: "u.FL",
		},
	},
	{
		// Broadcom AP621x 2.4 GHz only (brcmfmac), older Banana Pi boards.
		Match: []string{"Sinovoip BPI-M2", "BPI-M2-Zero", "BPI-M2-Ultra", "Banana Pi M2 Berry", "Banana Pi BPI-M3", "BananaPi-M64"},
		Info: DeviceInfo{
			Manufacturer: "Banana Pi", Model: "Onboard Wi-Fi (Broadcom AP621x)", Chipset: "BCM43430 (AP6212)",
			Bands: []string{"2.4 GHz"}, Standard: "802.11b/g/n",
			SupportsWPA3SAE: false, TxPowerAdjustable: false,
			MonitorBands: []string{}, InjectionBands: []string{},
			MaxChannelWidth: 20, HasDFS: false,
			Notes:               "Soldered Ampak AP6181/AP6212 (BCM43362/43430/43438, brcmfmac), 1x1. 2.4 GHz only, 802.11n. WPA2 AP. Base BPI-M2 has no Bluetooth.",
			StockAntennaGainDBI: 2, StockAntennaCount: 1, AntennaConnector: "u.FL",
		},
	},
}

// boardModel returns the system board/product identifier used to recognize
// soldered onboard Wi-Fi: the device-tree model on ARM SBCs, or the DMI
// vendor/product/board strings on x86. Empty if none is readable.
func boardModel() string {
	if b, err := os.ReadFile("/proc/device-tree/model"); err == nil {
		return strings.Trim(string(b), "\x00\n ")
	}
	var parts []string
	for _, f := range []string{
		"/sys/class/dmi/id/sys_vendor",
		"/sys/class/dmi/id/product_name",
		"/sys/class/dmi/id/board_name",
	} {
		if b, err := os.ReadFile(f); err == nil {
			if s := strings.TrimSpace(string(b)); s != "" && s != "To be filled by O.E.M." {
				parts = append(parts, s)
			}
		}
	}
	return strings.Join(parts, " ")
}

// LookupOnboard returns the curated DeviceInfo for this host's soldered Wi-Fi
// based on the board model, or nil if the board is unrecognized.
func LookupOnboard() *DeviceInfo {
	model := strings.ToLower(boardModel())
	if model == "" {
		return nil
	}
	for i := range onboardWiFiDB {
		for _, m := range onboardWiFiDB[i].Match {
			if strings.Contains(model, strings.ToLower(m)) {
				info := onboardWiFiDB[i].Info
				return &info
			}
		}
	}
	return nil
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
