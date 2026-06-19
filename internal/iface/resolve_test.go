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

// TestResolveInterfaceFreeMultiNetwork covers running several networks at once:
// each picks a free real adapter, a request for a busy adapter substitutes a free
// one, and when every adapter is in use the resolver errors cleanly (no crash).
func TestResolveInterfaceFreeMultiNetwork(t *testing.T) {
	adapters := []Adapter{
		{Interface: "wlan0", Driver: "rt2800usb"},
		{Interface: "wlan1", Driver: "mt76x2u"},
	}

	// First network claims its requested free adapter.
	got, sub, _, err := ResolveInterfaceFree(adapters, "wlan0", map[string]bool{})
	if err != nil || got != "wlan0" || sub {
		t.Errorf("free request: got %q sub=%v err=%v, want wlan0/false/nil", got, sub, err)
	}

	// Second network requests the busy adapter -> auto-substitutes the free one.
	got, sub, reason, err := ResolveInterfaceFree(adapters, "wlan0", map[string]bool{"wlan0": true})
	if err != nil || got != "wlan1" || !sub {
		t.Errorf("busy->free: got %q sub=%v err=%v, want wlan1/true/nil", got, sub, err)
	}
	if reason == "" {
		t.Error("expected a reason when substituting to a free adapter")
	}

	// Every adapter in use -> clear error, not a crash (the live gotcha result).
	if _, _, _, err = ResolveInterfaceFree(adapters, "wlan0", map[string]bool{"wlan0": true, "wlan1": true}); err == nil {
		t.Error("expected an error when every adapter is in use")
	}

	// Requested adapter free while another is busy -> use the requested one.
	got, sub, _, err = ResolveInterfaceFree(adapters, "wlan1", map[string]bool{"wlan0": true})
	if err != nil || got != "wlan1" || sub {
		t.Errorf("other-busy: got %q sub=%v err=%v, want wlan1/false/nil", got, sub, err)
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
