// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package portal

import (
	"strings"
	"testing"
)

func TestNormalizeRewritesForeignForm(t *testing.T) {
	in := `<!doctype html><html><body>
	  <form method="GET" action="https://login.realhotspot.example/auth">
	    <input type="text" name="j_username">
	    <input type="password" name="j_pwd">
	    <button type="submit">Log in</button>
	  </form>
	</body></html>`
	out := Normalize(in)

	if !strings.Contains(out, `action="/portal/accept"`) {
		t.Error("form action should be rewritten to /portal/accept")
	}
	if strings.Contains(out, "login.realhotspot.example") {
		t.Error("original foreign action should be gone")
	}
	if !strings.Contains(strings.ToLower(out), `method="post"`) {
		t.Error("method should be POST")
	}
	if !strings.Contains(out, `name="redirect"`) {
		t.Error("a hidden redirect field should be injected")
	}
	if !strings.Contains(out, `name="username"`) {
		t.Error("unrecognized username field should be retagged")
	}
	if !strings.Contains(out, `name="password"`) {
		t.Error("unrecognized password field should be retagged")
	}
}

func TestNormalizeIsIdempotent(t *testing.T) {
	in := `<html><body><form action="x"><input type="password" name="p"><input name="u"></form></body></html>`
	once := Normalize(in)
	twice := Normalize(once)
	if once != twice {
		t.Errorf("Normalize must be idempotent\nonce:  %s\ntwice: %s", once, twice)
	}
	// Exactly one redirect field after repeated runs.
	if n := strings.Count(twice, `name="redirect"`); n != 1 {
		t.Errorf("expected exactly 1 redirect field, got %d", n)
	}
}

func TestNormalizePreservesRecognizedNames(t *testing.T) {
	in := `<html><body><form action="/x"><input type="email" name="email"><input type="password" name="pass"></form></body></html>`
	out := Normalize(in)
	if !strings.Contains(out, `name="email"`) {
		t.Error("already-recognized email field must not be renamed")
	}
	if !strings.Contains(out, `name="pass"`) {
		t.Error("already-recognized pass field must not be renamed")
	}
	if strings.Contains(out, `name="username"`) {
		t.Error("should not add a username field when a recognized one exists")
	}
}

func TestNormalizeInjectsFormWhenNone(t *testing.T) {
	in := `<html><body><h1>Welcome to Free WiFi</h1></body></html>`
	out := Normalize(in)
	if !strings.Contains(out, `action="/portal/accept"`) {
		t.Error("a connect form should be injected when the page has none")
	}
}

func TestNormalizeEmptyAndGarbage(t *testing.T) {
	if Normalize("") != "" {
		t.Error("empty input should pass through")
	}
	// Non-HTML text gets wrapped by the parser but must not panic or be lost.
	if got := Normalize("just text"); !strings.Contains(got, "just text") {
		t.Error("text content should survive normalization")
	}
}
