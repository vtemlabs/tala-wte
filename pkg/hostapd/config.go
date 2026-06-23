// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

// Package hostapd builds hostapd configuration files and manages the hostapd process.
package hostapd

import (
	"bytes"
	"fmt"
	"os"
	"text/template"
)

// Protocol represents a wireless security protocol type.
type Protocol int

const (
	ProtocolOpen Protocol = iota
	ProtocolWPA
	ProtocolWPA2
	ProtocolWPS
	ProtocolWPA3
	ProtocolWPA3Transition
	ProtocolWPA2Enterprise
	ProtocolWPA3Enterprise
	ProtocolWEP
)

// PMFMode represents Protected Management Frame configuration.
type PMFMode int

const (
	PMFDisabled PMFMode = 0
	PMFOptional PMFMode = 1
	PMFRequired PMFMode = 2
)

// Config holds all parameters for generating a hostapd.conf file.
type Config struct {
	// Binary is the hostapd executable to run; empty means "hostapd" from PATH.
	// Opt-in vulnerable lab networks point this at the embedded patched build.
	Binary      string
	Interface   string
	SSID        string
	HWMode      string
	Channel     int
	CountryCode string // regulatory domain, e.g. "US"; required to beacon on 5 GHz
	Protocol    Protocol
	Passphrase  string
	WEPKey      string // pre-formatted wep_key0 value (bare hex, or "quoted ASCII")
	PMFMode     PMFMode
	APIsolate   bool
	Hidden      bool // ignore_broadcast_ssid: beacon an empty SSID so the network is not advertised
	IEEE80211N  bool
	IEEE80211AC bool
	IEEE80211AX bool
	OpClass     int // 131 for 6 GHz

	// WPS
	WPSPin     string
	DeviceName string

	// Enterprise / RADIUS
	RADIUSAddr   string
	RADIUSPort   int
	RADIUSSecret string
}

// WriteToTemp generates the config and writes to a temp file, returning the path.
func (c *Config) WriteToTemp() (string, error) {
	content, err := c.Generate()
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp("", "hostapd-*.conf")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		return "", err
	}
	return f.Name(), nil
}

// Generate produces the hostapd.conf content as a string.
func (c *Config) Generate() (string, error) {
	tmpl, err := template.New("hostapd").Parse(hostapdTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, c); err != nil {
		return "", fmt.Errorf("hostapd config template: %w", err)
	}
	return buf.String(), nil
}

var hostapdTemplate = `# Tala WTE - hostapd configuration
# Generated automatically - do not edit by hand

interface={{.Interface}}
driver=nl80211
ssid={{.SSID}}
hw_mode={{.HWMode}}
channel={{.Channel}}
{{- if .CountryCode}}
# Regulatory domain - without this the world domain ("00") marks 5 GHz
# channels "no IR" (no beaconing), so hostapd cannot start an AP on them.
country_code={{.CountryCode}}
ieee80211d=1
{{- end}}
{{- if .IEEE80211N}}
ieee80211n=1
{{- end}}
{{- if .IEEE80211AC}}
ieee80211ac=1
{{- end}}
{{- if .IEEE80211AX}}
ieee80211ax=1
{{- end}}
{{- if .OpClass}}
op_class={{.OpClass}}
{{- end}}
{{- if .APIsolate}}
ap_isolate=1
{{- end}}
{{- if .Hidden}}
# Hidden network: beacon an empty SSID and ignore broadcast-SSID probe requests.
ignore_broadcast_ssid=1
{{- end}}
wmm_enabled=1

{{- if eq .Protocol 0}}
# Open network - no authentication
{{- end}}

{{- if eq .Protocol 1}}
# WPA-Personal (TKIP)
wpa=1
wpa_passphrase={{.Passphrase}}
wpa_key_mgmt=WPA-PSK
wpa_pairwise=TKIP
group_cipher=TKIP
{{- end}}

{{- if eq .Protocol 2}}
# WPA2-Personal (CCMP/AES)
wpa=2
wpa_passphrase={{.Passphrase}}
wpa_key_mgmt=WPA-PSK
rsn_pairwise=CCMP
group_cipher=CCMP
{{- end}}

{{- if eq .Protocol 3}}
# WPS + WPA2
wpa=2
wpa_passphrase={{.Passphrase}}
wpa_key_mgmt=WPA-PSK
rsn_pairwise=CCMP
wps_state=2
eap_server=1
config_methods=label display push_button keypad
ap_setup_locked=0
{{- if .WPSPin}}
ap_pin={{.WPSPin}}
{{- end}}
device_name={{.DeviceName}}
manufacturer=TalaWTE
model_name=TalaWTE-AP
{{- end}}

{{- if eq .Protocol 4}}
# WPA3-Personal (SAE)
wpa=2
wpa_key_mgmt=SAE
rsn_pairwise=CCMP
ieee80211w={{.PMFMode}}
sae_password={{.Passphrase}}
sae_anti_clogging_threshold=5
sae_sync=5
{{- end}}

{{- if eq .Protocol 5}}
# WPA3-Transition (SAE + WPA2-PSK)
wpa=2
wpa_key_mgmt=SAE WPA-PSK WPA-PSK-SHA256
rsn_pairwise=CCMP
ieee80211w={{.PMFMode}}
sae_password={{.Passphrase}}
wpa_passphrase={{.Passphrase}}
transition_disable=0
{{- end}}

{{- if eq .Protocol 6}}
# WPA2-Enterprise (802.1X / EAP)
wpa=2
wpa_key_mgmt=WPA-EAP
rsn_pairwise=CCMP
ieee80211w={{.PMFMode}}
ieee8021x=1
eap_server=0
auth_server_addr={{.RADIUSAddr}}
auth_server_port={{.RADIUSPort}}
auth_server_shared_secret={{.RADIUSSecret}}
acct_server_addr={{.RADIUSAddr}}
acct_server_port=1813
acct_server_shared_secret={{.RADIUSSecret}}
eap_reauth_period=3600
{{- end}}

{{- if eq .Protocol 7}}
# WPA3-Enterprise (Suite-B-192)
wpa=2
wpa_key_mgmt=WPA-EAP-SUITE-B-192
rsn_pairwise=GCMP-256
ieee80211w={{.PMFMode}}
group_mgmt_cipher=BIP-GMAC-256
ieee8021x=1
eap_server=0
auth_server_addr={{.RADIUSAddr}}
auth_server_port={{.RADIUSPort}}
auth_server_shared_secret={{.RADIUSSecret}}
acct_server_addr={{.RADIUSAddr}}
acct_server_port=1813
acct_server_shared_secret={{.RADIUSSecret}}
{{- end}}

{{- if eq .Protocol 8}}
# WEP (DELIBERATELY INSECURE - for training/cracking labs only). WEP is
# incompatible with HT/VHT/HE, so 802.11n/ac/ax are intentionally not emitted.
# auth_algs=3 accepts both Open System and Shared Key auth so any WEP client
# associates (some, e.g. Android, default to Shared Key, which auth_algs=1 rejects
# with "Unsupported authentication algorithm").
auth_algs=3
wep_default_key=0
wep_key0={{.WEPKey}}
{{- end}}
`
