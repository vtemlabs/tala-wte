// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package client

import (
	"testing"

	"github.com/vtemlabs/tala-wte/internal/portal"
)

// TestAgentFillsEveryTemplate exercises all 36 built-in portal templates through
// the real agent form-fill (buildPortalSubmission) and the real engine validation
// (portal.MatchEntry). For each template it:
//   - resolves the template's assigned auth type,
//   - for a validating type, fills the template's actual form with a generated
//     credential and asserts the resulting submission validates against that
//     credential (so the auth type, the form fields, the alias map, and the agent
//     fill all agree),
//   - for any type, asserts the agent produces a non-empty submission so a member
//     can satisfy the portal.
func TestAgentFillsEveryTemplate(t *testing.T) {
	cat, err := portal.Catalog()
	if err != nil {
		t.Fatalf("load catalog: %v", err)
	}
	if len(cat) < 36 {
		t.Fatalf("expected at least 36 built-in templates, got %d", len(cat))
	}

	for i, tmpl := range cat {
		at := portal.AuthTypeForSlug(tmpl.Slug)
		spec := portal.Spec(at)
		creds := portal.GenerateEntry(at, i)

		action, values := buildPortalSubmission(tmpl.HTML, PortalConfig{Fields: creds})
		if action == "" {
			t.Errorf("%s (%s): no form action parsed", tmpl.Slug, at)
		}
		if len(values) == 0 {
			t.Errorf("%s (%s): agent produced an empty submission", tmpl.Slug, at)
			continue
		}

		if !spec.Validates {
			continue // non-validating: capture only, any non-empty submission is fine
		}

		submitted := map[string]string{}
		for k := range values {
			submitted[k] = values.Get(k)
		}
		if !portal.MatchEntry(at, submitted, creds) {
			t.Errorf("%s (%s): agent-filled submission does not validate.\n  creds=%v\n  submitted=%v",
				tmpl.Slug, at, creds, submitted)
		}
	}
}
