// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package iface

import "testing"

func TestResolveInterfacePrefersRequestedRealHardware(t *testing.T) {
	adapters := []Adapter{
		{Interface: "wlan0", Driver: "rt2800usb"},
		{Interface: "wlan1", Driver: "mac80211_hwsim"},
	}
	got, sub, _ := ResolveInterface(adapters, "wlan0")
	if got != "wlan0" {
		t.Errorf("expected wlan0, got %q", got)
	}
	if sub {
		t.Error("expected substituted=false when requested adapter is real")
	}
}

func TestResolveInterfaceSubstitutesVirtualWithReal(t *testing.T) {
	adapters := []Adapter{
		{Interface: "wlan0", Driver: "mac80211_hwsim"},
		{Interface: "wlan1", Driver: "rt2800usb"},
	}
	got, sub, reason := ResolveInterface(adapters, "wlan0")
	if got != "wlan1" {
		t.Errorf("expected substitution to wlan1, got %q", got)
	}
	if !sub {
		t.Error("expected substituted=true when requested adapter is virtual and real exists")
	}
	if reason == "" {
		t.Error("expected non-empty reason when substituting")
	}
}

func TestResolveInterfaceSubstitutesMissingWithReal(t *testing.T) {
	adapters := []Adapter{
		{Interface: "wlan0", Driver: "rt2800usb"},
	}
	got, sub, _ := ResolveInterface(adapters, "wlan99")
	if got != "wlan0" {
		t.Errorf("expected fallback to wlan0, got %q", got)
	}
	if !sub {
		t.Error("expected substituted=true when requested adapter not present")
	}
}

func TestResolveInterfaceFallsBackToVirtualWhenNoReal(t *testing.T) {
	adapters := []Adapter{
		{Interface: "wlan0", Driver: "mac80211_hwsim"},
	}
	got, sub, reason := ResolveInterface(adapters, "wlan0")
	if got != "wlan0" {
		t.Errorf("expected wlan0 (only option), got %q", got)
	}
	if sub {
		t.Error("expected substituted=false when no real hardware exists")
	}
	if reason == "" {
		t.Error("expected reason warning about no real hardware")
	}
}

func TestIsVirtualDriver(t *testing.T) {
	for _, d := range []string{"mac80211_hwsim", "virt_wifi"} {
		if !IsVirtualDriver(d) {
			t.Errorf("%q should be virtual", d)
		}
	}
	for _, d := range []string{"rt2800usb", "ath9k_htc", "mt7921u", ""} {
		if IsVirtualDriver(d) {
			t.Errorf("%q should not be virtual", d)
		}
	}
}

func TestBestRealAdapter(t *testing.T) {
	if got := BestRealAdapter(nil); got != nil {
		t.Errorf("nil input should return nil, got %+v", got)
	}
	if got := BestRealAdapter([]Adapter{{Driver: "mac80211_hwsim"}}); got != nil {
		t.Errorf("only-virtual input should return nil, got %+v", got)
	}
	got := BestRealAdapter([]Adapter{
		{Interface: "wlan0", Driver: "mac80211_hwsim"},
		{Interface: "wlan1", Driver: "rt2800usb"},
	})
	if got == nil || got.Interface != "wlan1" {
		t.Errorf("expected wlan1, got %+v", got)
	}
}
