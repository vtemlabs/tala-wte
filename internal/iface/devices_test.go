// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

package iface

import "testing"

// TestBrandForMACDistinguishesSharedUSBID covers the case that motivated OUI
// disambiguation: an ALFA and a generic MediaTek MT7921AU both enumerate as
// 0e8d:7961, so the USB ID alone cannot tell them apart - only the MAC OUI can.
func TestBrandForMACDistinguishesSharedUSBID(t *testing.T) {
	info := LookupDevice("0e8d:7961")
	if info == nil {
		t.Fatal("expected 0e8d:7961 in the device DB")
	}

	cases := []struct {
		name    string
		mac     string
		wantMfr string
		wantNot string
	}{
		{"ALFA OUI", "00:c0:ca:b7:ed:b4", "ALFA Network", ""},
		{"Panda OUI (explicit variant)", "9c:ef:d5:f6:35:e8", "Panda Wireless", "ALFA Network"},
		{"unknown MediaTek-family OUI falls to Panda variant", "00:0c:e7:11:22:33", "Panda Wireless", "ALFA Network"},
		{"unknown OUI falls back to canonical", "de:ad:be:ef:00:01", "ALFA Network", ""},
		{"empty MAC falls back to canonical", "", "ALFA Network", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotMfr, gotModel := info.BrandForMAC(tc.mac)
			if gotMfr != tc.wantMfr {
				t.Errorf("manufacturer: got %q, want %q", gotMfr, tc.wantMfr)
			}
			if tc.wantNot != "" && gotMfr == tc.wantNot {
				t.Errorf("manufacturer should not be %q for %s", tc.wantNot, tc.mac)
			}
			if gotModel == "" {
				t.Error("model should never be empty")
			}
		})
	}
}

// TestBrandForMACNoVariantsIsCanonical ensures single-product entries are
// untouched by the disambiguation.
func TestBrandForMACNoVariantsIsCanonical(t *testing.T) {
	info := LookupDevice("0bda:8812") // RTL8812AU, no variants
	if info == nil {
		t.Fatal("expected 0bda:8812 in the device DB")
	}
	mfr, model := info.BrandForMAC("9c:ef:d5:00:00:01")
	if mfr != info.Manufacturer || model != info.Model {
		t.Errorf("no-variant entry should keep canonical branding, got %q %q", mfr, model)
	}
}

func TestOUIVendor(t *testing.T) {
	if v := OUIVendor("00:C0:CA:11:22:33"); v != "ALFA Network" {
		t.Errorf("expected ALFA Network for 00:c0:ca (case-insensitive), got %q", v)
	}
	if v := OUIVendor("9c:ef:d5:00:00:00"); v != "MediaTek" {
		t.Errorf("expected MediaTek for 9c:ef:d5, got %q", v)
	}
	if v := OUIVendor("zz:zz"); v != "" {
		t.Errorf("expected empty for malformed MAC, got %q", v)
	}
}
