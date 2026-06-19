// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// See the LICENSE file.

package client

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/net/html"
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

// TestAllTemplatesFillable runs the filler against every built-in portal template
// and fails if any user-fillable form field is left empty - i.e. a template a
// member could not satisfy.
func TestAllTemplatesFillable(t *testing.T) {
	dir := "../portal/templates"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("templates dir unavailable: %v", err)
	}
	checked := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".html") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		_, vals := buildPortalSubmission(string(b), PortalConfig{})
		if missing := unfilledFields(string(b), vals); len(missing) > 0 {
			t.Errorf("%s: filler left fields empty: %v", e.Name(), missing)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no templates found to check")
	}
	t.Logf("filled every field in %d templates", checked)
}

// unfilledFields returns the named, user-fillable fields of the first form that
// the filler did not populate (submit/button/reset/image/file are ignored).
func unfilledFields(pageHTML string, vals url.Values) []string {
	doc, err := html.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return nil
	}
	form := findFirstForm(doc)
	if form == nil {
		return nil
	}
	var missing []string
	seen := map[string]bool{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			name := getNodeAttr(n, "name")
			switch n.Data {
			case "input":
				typ := strings.ToLower(getNodeAttr(n, "type"))
				skip := typ == "submit" || typ == "button" || typ == "reset" || typ == "image" || typ == "file"
				if name != "" && !skip && !seen[name] {
					seen[name] = true
					if strings.TrimSpace(vals.Get(name)) == "" {
						missing = append(missing, name)
					}
				}
			case "select", "textarea":
				if name != "" && !seen[name] {
					seen[name] = true
					if strings.TrimSpace(vals.Get(name)) == "" {
						missing = append(missing, name)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(form)
	return missing
}
