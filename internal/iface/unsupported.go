// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

package iface

import (
	"os"
	"path/filepath"
	"strings"
)

// wifiUSBVendors maps USB vendor IDs of common Wi-Fi chip/adapter makers to a
// display name. A USB device on one of these vendors that has no ieee80211 phy is
// almost certainly a wireless adapter whose driver or firmware is missing.
var wifiUSBVendors = map[string]string{
	"0e8d": "MediaTek", "148f": "Ralink/MediaTek", "0bda": "Realtek",
	"0cf3": "Qualcomm Atheros", "168c": "Atheros", "8087": "Intel", "8086": "Intel",
	"0846": "NETGEAR", "2357": "TP-Link", "2cf0": "Panda Wireless", "0b05": "ASUS",
	"13b1": "Linksys", "0411": "Buffalo", "7392": "Edimax", "0489": "Foxconn/MediaTek",
	"050d": "Belkin", "157e": "TRENDnet", "20f4": "TRENDnet", "0bdaff": "Realtek",
}

// UnsupportedAdapter is a USB wireless device that is physically present but has
// no working driver/firmware (no ieee80211 phy), so the operator needs to find
// and install a driver for it before it can be used.
type UnsupportedAdapter struct {
	USBID  string `json:"usb_id"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// UnsupportedAdapters scans USB for wireless devices that have no working phy
// (driver or firmware not loaded). The installer warns about these and the app
// surfaces them so the operator knows a driver is needed.
func UnsupportedAdapters() []UnsupportedAdapter {
	out := []UnsupportedAdapter{}
	entries, err := os.ReadDir("/sys/bus/usb/devices")
	if err != nil {
		return out
	}
	for _, e := range entries {
		name := e.Name()
		if strings.Contains(name, ":") { // skip interface entries (e.g. 2-3:1.0)
			continue
		}
		dir := filepath.Join("/sys/bus/usb/devices", name)
		vendor := strings.ToLower(readSysAttr(filepath.Join(dir, "idVendor")))
		product := strings.ToLower(readSysAttr(filepath.Join(dir, "idProduct")))
		if vendor == "" {
			continue
		}
		vName, isWifiVendor := wifiUSBVendors[vendor]
		if !isWifiVendor {
			continue
		}
		// Supported if a Wi-Fi driver is bound (covers a healthy radio, a radio
		// whose phy is currently inside a running network's namespace, and a wedge
		// the heal will recover) or a phy is already present. Only a device with no
		// driver bound and no phy is a failed probe needing a driver/firmware.
		if usbDeviceHasPhy(dir) || usbDeviceHasWifiDriver(dir) {
			continue
		}
		id := vendor + ":" + product
		label := vName + " wireless adapter (" + id + ")"
		if info, ok := wirelessDeviceDB[id]; ok {
			label = info.Manufacturer + " " + info.Model // recognized model, just no driver/firmware
		}
		out = append(out, UnsupportedAdapter{
			USBID:  id,
			Name:   label,
			Reason: "no driver/firmware loaded; install the driver for this adapter",
		})
	}
	return out
}

// usbDeviceHasPhy reports whether any interface of a USB device has an ieee80211
// phy or a network interface (i.e. its Wi-Fi driver is bound and firmware loaded).
func usbDeviceHasPhy(devDir string) bool {
	ifaces, _ := filepath.Glob(devDir + ":*")
	for _, i := range ifaces {
		if m, _ := filepath.Glob(i + "/ieee80211/phy*"); len(m) > 0 {
			return true
		}
		if m, _ := filepath.Glob(i + "/net/*"); len(m) > 0 {
			return true
		}
	}
	return false
}

// knownWifiDrivers are the USB Wi-Fi drivers Tala WTE expects to bind. A device
// bound to one of these is supported even if its phy is momentarily absent from
// the host (moved into a network namespace, or mid firmware-init recovery).
var knownWifiDrivers = map[string]bool{
	"mt7921u": true, "mt76x2u": true, "mt76x0u": true, "mt76_usb": true,
	"rt2800usb": true, "rt2x00usb": true, "rtl8xxxu": true, "rtl88xxau": true,
	"rtl8812au": true, "rtl8814au": true, "8188eu": true, "8188eus": true,
	"rtw_8822bu": true, "ath9k_htc": true, "carl9170": true, "brcmfmac": true,
	"zd1211rw": true, "mwifiex": true, "rndis_wlan": true,
}

// usbDeviceHasWifiDriver reports whether any interface of a USB device is bound
// to a known Wi-Fi driver.
func usbDeviceHasWifiDriver(devDir string) bool {
	ifaces, _ := filepath.Glob(devDir + ":*")
	for _, i := range ifaces {
		link, err := os.Readlink(filepath.Join(i, "driver"))
		if err != nil {
			continue
		}
		if knownWifiDrivers[filepath.Base(link)] {
			return true
		}
	}
	return false
}

func readSysAttr(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
