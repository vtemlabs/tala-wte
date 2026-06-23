// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package sim

import (
	"os"
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
	cfg := buildConfig(rec, "wlan0", "")

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
	cfg := buildConfig(rec, "wlan0", "")

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
		cfg := buildConfig(rec, "wlan0", "")
		if cfg.Binary != "" {
			t.Fatalf("protocol %q must not set a custom hostapd binary, got %q", proto, cfg.Binary)
		}
	}
}
