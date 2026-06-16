// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

package ldap

import (
	"strings"
	"testing"
)

// TestGenerateMixedPasswordVariety verifies that across a 50-user directory
// the generator produces a meaningful spread - not a single password reused
// for every account, and not a single tier dominating completely.
func TestGenerateMixedPasswordVariety(t *testing.T) {
	const n = 50
	seen := make(map[string]int)
	weakHits := 0
	strongHits := 0

	for i := 0; i < n; i++ {
		first := randomFirstNames[i%len(randomFirstNames)]
		last := randomLastNames[i%len(randomLastNames)]
		pw := generateMixedPassword(first, last, "ACME Corp")
		seen[pw]++
		// Strong tier passwords are 12 alphanumerics+symbols with no human
		// readable substring; we use the simple heuristic "no firstname / lastname
		// substring AND no weakCommon match".
		if isWeakCommon(pw) {
			weakHits++
		}
		if !isWeakCommon(pw) && !strings.Contains(pw, first) && !strings.Contains(pw, last) &&
			!strings.Contains(pw, strings.ToLower(first)) && !strings.Contains(pw, strings.ToLower(last)) &&
			!strings.Contains(pw, "Spring") && !strings.Contains(pw, "Summer") &&
			!strings.Contains(pw, "Fall") && !strings.Contains(pw, "Winter") &&
			!strings.HasPrefix(pw, "ACME") {
			strongHits++
		}
	}

	if len(seen) < n/3 {
		t.Errorf("expected at least %d unique passwords across %d users, got %d (insufficient variety)", n/3, n, len(seen))
	}

	// With 40% weak / 30% strong distribution and 50 samples, both buckets
	// should hit at least 5 times - anything below suggests a busted weight.
	if weakHits < 5 {
		t.Errorf("expected at least 5 weak-tier passwords across %d users, got %d", n, weakHits)
	}
	if strongHits < 5 {
		t.Errorf("expected at least 5 strong-tier passwords across %d users, got %d", n, strongHits)
	}
}

func TestGenerateMixedPasswordNoEmpty(t *testing.T) {
	for i := 0; i < 20; i++ {
		pw := generateMixedPassword("Alice", "Smith", "Contoso")
		if pw == "" {
			t.Fatal("generator produced an empty password")
		}
		if len(pw) < 6 {
			t.Errorf("password %q is suspiciously short (len %d)", pw, len(pw))
		}
	}
}

func TestTitleCase(t *testing.T) {
	cases := map[string]string{
		"":      "",
		"a":     "A",
		"AB":    "Ab",
		"alice": "Alice",
		"BOB":   "Bob",
	}
	for in, want := range cases {
		if got := titleCase(in); got != want {
			t.Errorf("titleCase(%q) = %q, want %q", in, got, want)
		}
	}
}

func isWeakCommon(pw string) bool {
	for _, w := range weakCommonPasswords {
		if pw == w {
			return true
		}
	}
	return false
}
