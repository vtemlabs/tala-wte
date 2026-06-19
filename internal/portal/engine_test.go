// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// See the LICENSE file.

package portal

import (
	"strings"
	"testing"
)

// TestDefaultPortalAuth verifies the built-in portal renders a login form when
// the network requires authentication, and the accept-only splash otherwise.
func TestDefaultPortalAuth(t *testing.T) {
	auth := defaultPortal(true)
	if !strings.Contains(auth, `name="username"`) || !strings.Contains(auth, `name="password"`) {
		t.Errorf("auth default portal must have username + password fields, got:\n%s", auth)
	}

	open := defaultPortal(false)
	if strings.Contains(open, `name="password"`) {
		t.Error("non-auth default portal must not have a password field")
	}
	if !strings.Contains(open, "Connect to Internet") {
		t.Error("non-auth default portal should be the accept-only splash")
	}

	// The auth portal's fields must be ones the capture endpoint recognizes.
	u, p := extractCreds(map[string][]string{"username": {"x"}, "password": {"y"}})
	if u == "" || p == "" {
		t.Error("capture endpoint should recognize the default auth portal's field names")
	}
}
