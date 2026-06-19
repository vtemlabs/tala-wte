// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package ldap

import "testing"

// TestBuildGroupsRealistic verifies the generated directory has the department,
// privilege, and access groups a real domain would, that every group is valid
// (groupOfNames requires at least one member), and that no member is fabricated.
func TestBuildGroupsRealistic(t *testing.T) {
	users := []ProvisionUser{
		{UID: "asmith", CN: "Alice Smith", Department: "Information Technology", Title: "IT Manager"},
		{UID: "bjones", CN: "Bob Jones", Department: "Engineering", Title: "Software Engineer"},
		{UID: "cwilliams", CN: "Carol Williams", Department: "Engineering", Title: "QA Engineer"},
		{UID: "dlee", CN: "Dan Lee", Department: "Executive", Title: "Chief Executive Officer"},
		{UID: "esun", CN: "Eve Sun", Department: "Sales", Title: "Account Executive"},
	}
	groups := buildGroups(users)
	if len(groups) == 0 {
		t.Fatal("expected at least one group")
	}

	valid := map[string]bool{}
	for _, u := range users {
		valid[u.UID] = true
	}
	byCN := map[string]provisionGroup{}
	for _, g := range groups {
		if len(g.Members) == 0 {
			t.Errorf("group %q has no members; groupOfNames requires >=1", g.CN)
		}
		for _, m := range g.Members {
			if !valid[m] {
				t.Errorf("group %q has unknown member %q", g.CN, m)
			}
		}
		byCN[g.CN] = g
	}

	for _, want := range []string{"Domain Users", "Domain Admins", "Information Technology", "Engineering", "Sales", "wifi-users", "wifi-admins"} {
		if _, ok := byCN[want]; !ok {
			t.Errorf("expected a %q group", want)
		}
	}
	if got := len(byCN["Domain Users"].Members); got != len(users) {
		t.Errorf("Domain Users should hold all %d users, got %d", len(users), got)
	}
	if n := len(byCN["Domain Admins"].Members); n == 0 || n > 3 {
		t.Errorf("Domain Admins should be 1-3 members, got %d", n)
	}
	// An empty department must not produce a group.
	if _, ok := byCN["Legal"]; ok {
		t.Error("did not expect a Legal group when no user is in Legal")
	}
}

// TestBuildGroupsAdminFallback ensures Domain Admins is never empty even when no
// IT or executive user exists.
func TestBuildGroupsAdminFallback(t *testing.T) {
	users := []ProvisionUser{{UID: "asmith", CN: "Alice Smith", Department: "Sales", Title: "Sales Representative"}}
	var admins provisionGroup
	for _, g := range buildGroups(users) {
		if g.CN == "Domain Admins" {
			admins = g
		}
	}
	if len(admins.Members) != 1 || admins.Members[0] != "asmith" {
		t.Errorf("expected Domain Admins to fall back to [asmith], got %v", admins.Members)
	}
}

// TestGenerateUsersAssignsDeptAndUniqueUID checks every generated user gets a
// department and title, and that uids do not collide.
func TestGenerateUsersAssignsDeptAndUniqueUID(t *testing.T) {
	users := generateUsers(ProvisionRequest{CompanyName: "Test Co", Domain: "test.local", UserCount: 30})
	if len(users) != 30 {
		t.Fatalf("want 30 users, got %d", len(users))
	}
	seen := map[string]bool{}
	for _, u := range users {
		if u.Department == "" {
			t.Errorf("user %s has no department", u.UID)
		}
		if u.Title == "" {
			t.Errorf("user %s has no title", u.UID)
		}
		if seen[u.UID] {
			t.Errorf("duplicate uid %q", u.UID)
		}
		seen[u.UID] = true
	}
}
