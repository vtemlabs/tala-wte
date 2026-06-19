// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. See the LICENSE file.

package iface

import (
	"reflect"
	"testing"
)

// TestComputeLimits checks the plain-language capability limits derived for the
// curated adapters, including the empirically-confirmed WPA3-SAE split (MT76 yes,
// legacy Ralink no).
func TestComputeLimits(t *testing.T) {
	cases := []struct {
		name string
		a    Adapter
		want []string
	}{
		{
			name: "MT7612U dual-band AC is unrestricted",
			a:    Adapter{USBID: "0e8d:7612", Bands: []string{"2.4 GHz", "5 GHz"}, InjectionBands: []string{"2.4 GHz", "5 GHz"}, MaxChannelWidth: 80},
			want: nil,
		},
		{
			name: "RT5572 legacy: no SAE + narrow width",
			a:    Adapter{USBID: "148f:5572", Bands: []string{"2.4 GHz", "5 GHz"}, InjectionBands: []string{"2.4 GHz", "5 GHz"}, MaxChannelWidth: 40},
			want: []string{"No WPA3-SAE (legacy chipset)", "Max channel width 40 MHz (no 80/160 MHz)"},
		},
		{
			name: "RT3070 is 2.4 GHz only and legacy",
			a:    Adapter{USBID: "148f:3070", Bands: []string{"2.4 GHz"}, InjectionBands: []string{"2.4 GHz"}, MaxChannelWidth: 40},
			want: []string{"2.4 GHz only (no 5/6 GHz)", "No WPA3-SAE (legacy chipset)", "Max channel width 40 MHz (no 80/160 MHz)"},
		},
		{
			name: "MT7921AU: 6 GHz is client/monitor only",
			a:    Adapter{USBID: "0e8d:7961", Bands: []string{"2.4 GHz", "5 GHz", "6 GHz"}, APBands: []string{"2.4 GHz", "5 GHz"}, InjectionBands: []string{"2.4 GHz", "5 GHz"}, MaxChannelWidth: 160},
			want: []string{"No 6 GHz AP (6 GHz is client/monitor only)"},
		},
		{
			name: "unknown adapter is never wrongly flagged",
			a:    Adapter{USBID: "", Bands: nil, MaxChannelWidth: 0},
			want: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := computeLimits(c.a)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("computeLimits() = %v, want %v", got, c.want)
			}
		})
	}
}
