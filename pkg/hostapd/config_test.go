// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.

package hostapd

import (
	"strings"
	"testing"
)

// directiveValue returns the value of the first "key=value" line, and whether
// the key was present at all.
func directiveValue(cfg, key string) (string, bool) {
	for _, line := range strings.Split(cfg, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key+"=") {
			return strings.TrimPrefix(line, key+"="), true
		}
	}
	return "", false
}

// A WPS network MUST emit a complete, non-empty WPS device profile. hostapd
// silently refuses to initialize WPS (no WSC EAP method is offered, so an
// attacker only ever sees "EAP type 0 (unknown)" and never starts the M1/M3
// exchange) when device_name is empty or the device attributes are missing.
// This guards the regression that empty device_name caused.
func TestGenerateWPSHasDeviceProfile(t *testing.T) {
	c := &Config{
		Interface:  "wlan0",
		SSID:       "wlab-wps-pixie",
		HWMode:     "g",
		Channel:    1,
		Protocol:   ProtocolWPS,
		Passphrase: "labsecret123",
		WPSPin:     "77080165",
	}
	out, err := c.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	required := []string{
		"wps_state", "eap_server", "config_methods", "ap_setup_locked",
		"device_name", "manufacturer", "model_name", "model_number",
		"serial_number", "device_type",
	}
	for _, key := range required {
		v, ok := directiveValue(out, key)
		if !ok {
			t.Errorf("WPS config missing %q directive\n---\n%s", key, out)
			continue
		}
		if strings.TrimSpace(v) == "" {
			t.Errorf("WPS directive %q is empty (hostapd disables WPS) \n---\n%s", key, out)
		}
	}

	if v, _ := directiveValue(out, "ap_pin"); v != "77080165" {
		t.Errorf("ap_pin = %q, want the configured PIN (online registrar target)", v)
	}
}

// A non-WPS network must not emit any WPS directives.
func TestGenerateWPA2NoWPS(t *testing.T) {
	c := &Config{Interface: "wlan0", SSID: "x", HWMode: "g", Channel: 1, Protocol: ProtocolWPA2, Passphrase: "labsecret123"}
	out, err := c.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, key := range []string{"wps_state", "ap_pin", "device_name"} {
		if _, ok := directiveValue(out, key); ok {
			t.Errorf("WPA2 config unexpectedly contains WPS directive %q", key)
		}
	}
}

// OWE (Enhanced Open) must use OWE key management with mandatory PMF and carry no
// passphrase (it is unauthenticated, encrypted via Diffie-Hellman).
func TestGenerateOWE(t *testing.T) {
	c := &Config{Interface: "wlan0", SSID: "wlab-owe", HWMode: "g", Channel: 6, Protocol: ProtocolOWE}
	out, err := c.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, want := range []string{"wpa_key_mgmt=OWE", "rsn_pairwise=CCMP", "ieee80211w=2"} {
		if !strings.Contains(out, want) {
			t.Errorf("OWE config missing %q\n---\n%s", want, out)
		}
	}
	if strings.Contains(out, "wpa_passphrase") || strings.Contains(out, "sae_password") {
		t.Errorf("OWE config must not carry a passphrase\n---\n%s", out)
	}
}

// WPA2-PSK + 802.11r must advertise FT-PSK key management and a mobility domain.
func TestGenerateFT(t *testing.T) {
	c := &Config{Interface: "wlan0", SSID: "wlab-ft", HWMode: "g", Channel: 6, Protocol: ProtocolWPA2FT, Passphrase: "labsecret123"}
	out, err := c.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, want := range []string{"wpa_key_mgmt=FT-PSK WPA-PSK", "mobility_domain=", "ft_psk_generate_local=1", "wpa_passphrase=labsecret123"} {
		if !strings.Contains(out, want) {
			t.Errorf("FT config missing %q\n---\n%s", want, out)
		}
	}
}

// OWE-transition must emit an open primary BSS plus a hidden companion OWE BSS that
// cross-reference each other by BSSID and SSID (the downgrade target).
func TestGenerateOWETransition(t *testing.T) {
	c := &Config{
		Interface: "wlan0", SSID: "wlab-owe-trans", HWMode: "g", Channel: 6,
		Protocol:          ProtocolOWETransition,
		RadioMAC:          "00:c0:ca:b2:f4:b8",
		OWESecondaryBSSID: "00:c0:ca:b2:f4:b9",
		OWEHiddenSSID:     "wlab-owe-trans-enc",
	}
	out, err := c.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, want := range []string{
		"owe_transition_bssid=00:c0:ca:b2:f4:b9",
		`owe_transition_ssid="wlab-owe-trans-enc"`,
		"bss=owetr0",
		"bssid=00:c0:ca:b2:f4:b9",
		"wpa_key_mgmt=OWE",
		"ieee80211w=2",
		"owe_transition_bssid=00:c0:ca:b2:f4:b8",
		`owe_transition_ssid="wlab-owe-trans"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("OWE-transition config missing %q\n---\n%s", want, out)
		}
	}
}
