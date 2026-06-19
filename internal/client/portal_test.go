// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// See the LICENSE file.

package client

import (
	"strings"
	"testing"
)

// TestBuildPortalSubmission feeds the filler a form of each template shape and
// confirms it produces the fields that template actually requires.
func TestBuildPortalSubmission(t *testing.T) {
	cases := []struct {
		name     string
		html     string
		nonEmpty []string
		want     map[string]string
	}{
		{
			name:     "login AD/azuread",
			html:     `<form action="/portal/accept"><input name="username" type="email"><input name="password" type="password"><button type="submit">Sign in</button></form>`,
			nonEmpty: []string{"username", "password"},
		},
		{
			name:     "PII collection",
			html:     `<form action="/portal/accept"><input name="first_name"><input name="last_name"><input name="email" type="email"><input name="postal_code"></form>`,
			nonEmpty: []string{"first_name", "last_name", "email", "postal_code"},
		},
		{
			name:     "radio plan + terms checkbox",
			html:     `<form action="/portal/accept"><input type="hidden" name="plan" value="Free"><input type="radio" name="choice" value="Free"><input type="radio" name="choice" value="Premium"><input type="checkbox" name="agree"><button>Connect</button></form>`,
			nonEmpty: []string{"choice", "agree"},
			want:     map[string]string{"plan": "Free"},
		},
		{
			name: "select dropdown skips placeholder",
			html: `<form action="/portal/accept"><select name="room_type"><option value="">Select...</option><option value="standard">Standard</option><option value="suite">Suite</option></select></form>`,
			want: map[string]string{"room_type": "standard"},
		},
		{
			name: "accept-only hidden field",
			html: `<form action="/portal/accept"><input type="hidden" name="accepted_terms" value="yes"><button>Continue</button></form>`,
			want: map[string]string{"accepted_terms": "yes"},
		},
		{
			name:     "hotel tabbed room + method",
			html:     `<form action="/portal/accept"><input name="last_name"><input name="room_number"><input type="hidden" name="method" value="guest"></form>`,
			nonEmpty: []string{"last_name", "room_number"},
			want:     map[string]string{"method": "guest"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, vals := buildPortalSubmission(c.html, PortalConfig{})
			for _, k := range c.nonEmpty {
				if vals.Get(k) == "" {
					t.Errorf("field %q should be filled, got empty (all: %v)", k, vals)
				}
			}
			for k, want := range c.want {
				if got := vals.Get(k); got != want {
					t.Errorf("field %q = %q, want %q", k, got, want)
				}
			}
		})
	}
}

// TestPortalIdentity verifies harvested creds look realistic and that operator
// credentials override the generated identity.
func TestPortalIdentity(t *testing.T) {
	_, vals := buildPortalSubmission(`<form action="/portal/accept"><input name="email" type="email"><input name="password" type="password"></form>`, PortalConfig{})
	if !strings.Contains(vals.Get("email"), "@") {
		t.Errorf("email should be AD-style, got %q", vals.Get("email"))
	}
	if len(vals.Get("password")) < 8 {
		t.Errorf("password should be non-trivial, got %q", vals.Get("password"))
	}

	_, vals2 := buildPortalSubmission(`<form action="/portal/accept"><input name="username"><input name="password" type="password"></form>`, PortalConfig{Username: "opuser", Password: "oppass1234"})
	if vals2.Get("username") != "opuser" || vals2.Get("password") != "oppass1234" {
		t.Errorf("operator creds should win: got %q / %q", vals2.Get("username"), vals2.Get("password"))
	}
}
