// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package sim

import (
	"os"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"

	"github.com/vtemlabs/tala-wte/pkg/hostapd"
)

func newNetworkRecord(fields map[string]any) *core.Record {
	col := core.NewBaseCollection("networks")
	col.Fields.Add(
		&core.TextField{Name: "ssid"},
		&core.SelectField{Name: "protocol", MaxSelect: 1, Values: []string{
			"open", "wep", "wpa", "wpa2", "wps", "wpa3", "wpa3_transition",
			"wpa2_enterprise", "wpa3_enterprise",
		}},
		&core.TextField{Name: "band"},
		&core.NumberField{Name: "channel"},
		&core.TextField{Name: "passphrase"},
		&core.BoolField{Name: "client_isolation"},
		&core.BoolField{Name: "hidden"},
		&core.BoolField{Name: "wps_pixie"},
		&core.BoolField{Name: "pmkid_exposed"},
	)
	rec := core.NewRecord(col)
	for k, v := range fields {
		rec.Set(k, v)
	}
	return rec
}

// A WPS network with the downgrade toggle on must run the embedded
// Pixie-Dust-vulnerable hostapd, and that binary must actually be extracted.
func TestBuildConfigWPSPixieDowngradeOn(t *testing.T) {
	rec := newNetworkRecord(map[string]any{
		"ssid": "wlab-wps", "protocol": "wps", "band": "2.4", "channel": 1, "wps_pixie": true,
	})
	cfg := buildConfig(rec, "wlan0", "", "")

	if cfg.Protocol != hostapd.ProtocolWPS {
		t.Fatalf("protocol = %v, want WPS", cfg.Protocol)
	}
	if cfg.WPSPin == "" {
		t.Fatal("WPS network must set an ap_pin (online registrar target)")
	}
	if cfg.Binary == "" {
		t.Fatal("downgraded WPS must select the embedded patched hostapd, got empty Binary")
	}
	if fi, err := os.Stat(cfg.Binary); err != nil {
		t.Fatalf("embedded hostapd not extracted at %q: %v", cfg.Binary, err)
	} else if fi.Size() == 0 {
		t.Fatalf("extracted hostapd is empty at %q", cfg.Binary)
	}
}

// By default a WPS network is Pixie-resistant: it keeps the ap_pin (online PIN
// target) but runs system hostapd, so cfg.Binary stays empty.
func TestBuildConfigWPSPixieDowngradeOff(t *testing.T) {
	rec := newNetworkRecord(map[string]any{
		"ssid": "wlab-wps", "protocol": "wps", "band": "2.4", "channel": 1, "wps_pixie": false,
	})
	cfg := buildConfig(rec, "wlan0", "", "")

	if cfg.WPSPin == "" {
		t.Fatal("WPS network must set an ap_pin even without the downgrade")
	}
	if cfg.Binary != "" {
		t.Fatalf("non-downgraded WPS must use system hostapd, got Binary = %q", cfg.Binary)
	}
}

// Non-WPS protocols never select a custom hostapd binary.
func TestBuildConfigNonWPSNoCustomBinary(t *testing.T) {
	for _, proto := range []string{"open", "wpa2", "wpa3", "wep"} {
		rec := newNetworkRecord(map[string]any{
			"ssid": "x", "protocol": proto, "band": "2.4", "channel": 6, "wps_pixie": true,
		})
		cfg := buildConfig(rec, "wlan0", "", "")
		if cfg.Binary != "" {
			t.Fatalf("protocol %q must not set a custom hostapd binary, got %q", proto, cfg.Binary)
		}
	}
}

// A WPA2 network with PMKID exposed must run the embedded patched hostapd
// (which advertises the RSN PMKID KDE in M1), and that binary must be extracted.
func TestBuildConfigPMKIDExposedOn(t *testing.T) {
	rec := newNetworkRecord(map[string]any{
		"ssid": "wlab-pmkid", "protocol": "wpa2", "band": "2.4", "channel": 6,
		"passphrase": "labsecret123", "pmkid_exposed": true,
	})
	cfg := buildConfig(rec, "wlan0", "", "")

	if cfg.Protocol != hostapd.ProtocolWPA2 {
		t.Fatalf("protocol = %v, want WPA2", cfg.Protocol)
	}
	if cfg.Binary == "" {
		t.Fatal("PMKID-exposed WPA2 must select the embedded patched hostapd, got empty Binary")
	}
	if fi, err := os.Stat(cfg.Binary); err != nil {
		t.Fatalf("embedded hostapd not extracted at %q: %v", cfg.Binary, err)
	} else if fi.Size() == 0 {
		t.Fatalf("extracted hostapd is empty at %q", cfg.Binary)
	}
}

// By default a WPA2 network withholds the PMKID: it uses system hostapd, so
// cfg.Binary stays empty.
func TestBuildConfigPMKIDExposedOff(t *testing.T) {
	rec := newNetworkRecord(map[string]any{
		"ssid": "wlab-wpa2", "protocol": "wpa2", "band": "2.4", "channel": 6,
		"passphrase": "labsecret123", "pmkid_exposed": false,
	})
	cfg := buildConfig(rec, "wlan0", "", "")
	if cfg.Binary != "" {
		t.Fatalf("non-exposed WPA2 must use system hostapd, got Binary = %q", cfg.Binary)
	}
}

// A crafted SSID or passphrase containing control characters must not inject a
// hostapd directive: control chars are stripped before they reach the config.
func TestBuildConfigStripsControlChars(t *testing.T) {
	rec := newNetworkRecord(map[string]any{
		"ssid": "evil\nap_setup_locked=0", "protocol": "wpa2", "band": "2.4", "channel": 6,
		"passphrase": "secret\nignore_broadcast_ssid=1",
	})
	cfg := buildConfig(rec, "wlan0", "", "")
	if strings.ContainsAny(cfg.SSID, "\n\r") {
		t.Errorf("SSID not sanitized: %q", cfg.SSID)
	}
	if strings.ContainsAny(cfg.Passphrase, "\n\r") {
		t.Errorf("passphrase not sanitized: %q", cfg.Passphrase)
	}
	out, err := cfg.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, injected := range []string{"ap_setup_locked=0", "ignore_broadcast_ssid=1"} {
		if _, ok := directiveLine(out, injected); ok {
			t.Errorf("crafted value injected %q as a directive\n---\n%s", injected, out)
		}
	}
}

// Regression: the WEP key path once read the raw record value, so a 13-rune key
// with embedded newlines injected directives. It must use the sanitized value.
func TestBuildConfigWEPKeySanitized(t *testing.T) {
	// 13 runes including two newlines: the length fitWEPKey keeps verbatim.
	rec := newNetworkRecord(map[string]any{
		"ssid": "wlab-wep", "protocol": "wep", "band": "2.4", "channel": 6,
		"passphrase": "abc\nwpa=2\ndef",
	})
	cfg := buildConfig(rec, "wlan0", "", "")
	if strings.ContainsAny(cfg.WEPKey, "\n\r") {
		t.Errorf("WEP key carried a control character: %q", cfg.WEPKey)
	}
	out, err := cfg.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if _, ok := directiveLine(out, "wpa=2"); ok {
		t.Errorf("WEP key newline injected a wpa=2 directive\n---\n%s", out)
	}
}

// directiveLine reports whether the generated config has a standalone line equal
// to want (a full injected directive), ignoring surrounding whitespace.
func directiveLine(cfg, want string) (string, bool) {
	for _, line := range strings.Split(cfg, "\n") {
		if strings.TrimSpace(line) == want {
			return line, true
		}
	}
	return "", false
}
