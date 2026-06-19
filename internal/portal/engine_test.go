// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// See the LICENSE file.

package portal

import (
	"net/http/httptest"
	"net/url"
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

// TestSubmissionFieldsPackMember verifies the X-Tala-Member header is recorded as
// _pack_member, control fields are dropped, and a target submission is untagged.
func TestSubmissionFieldsPackMember(t *testing.T) {
	form := url.Values{"username": {"jsmith"}, "password": {"Summer2026!"}, "redirect": {"http://x"}}

	req := httptest.NewRequest("POST", "/portal/accept", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Tala-Member", "lab-client-1")
	fields, _ := submissionFields(req, false, nil)
	if fields["_pack_member"] != "lab-client-1" {
		t.Errorf("_pack_member = %q, want lab-client-1", fields["_pack_member"])
	}
	if _, ok := fields["redirect"]; ok {
		t.Error("redirect is a control field and should not be harvested")
	}
	if fields["username"] != "jsmith" {
		t.Errorf("username should be harvested, got %q", fields["username"])
	}

	// No header -> untagged (a real target).
	req2 := httptest.NewRequest("POST", "/portal/accept", strings.NewReader(form.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	f2, _ := submissionFields(req2, false, nil)
	if _, ok := f2["_pack_member"]; ok {
		t.Error("a target submission should not be tagged pack member")
	}
}
