// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package portal

import "testing"

// validatingTypes is every auth type that checks a credential set.
var validatingTypes = []AuthType{
	AuthUserPassword, AuthEmailPassword, AuthHotel, AuthVoucher, AuthMembership,
}

// TestGeneratedCredentialMatchesItself: a generated entry must validate against
// itself, and a wrong value for any key field must be rejected, for every type.
func TestGeneratedCredentialMatchesItself(t *testing.T) {
	for _, at := range validatingTypes {
		entry := GenerateEntry(at, 3)
		if len(entry) == 0 {
			t.Fatalf("%s: GenerateEntry produced no fields", at)
		}
		if !MatchEntry(at, entry, entry) {
			t.Errorf("%s: a generated entry %v did not validate against itself", at, entry)
		}
		// Flip each key field; the submission must then fail.
		for _, k := range Spec(at).KeyFields() {
			bad := map[string]string{}
			for kk, vv := range entry {
				bad[kk] = vv
			}
			bad[k] = entry[k] + "-WRONG"
			if MatchEntry(at, bad, entry) {
				t.Errorf("%s: submission with wrong %s should not match", at, k)
			}
		}
		// An empty submission never matches.
		if MatchEntry(at, map[string]string{}, entry) {
			t.Errorf("%s: empty submission should not match", at)
		}
	}
}

// TestNonValidatingTypes: click-through / email / info form do not validate.
func TestNonValidatingTypes(t *testing.T) {
	for _, at := range []AuthType{AuthClickThrough, AuthEmail, AuthInfoForm} {
		if Spec(at).Validates {
			t.Errorf("%s should not be a validating type", at)
		}
	}
}

// TestAliasMatching: a canonical credential set validates a submission that uses a
// template's own field names (stateroom, voucher, account, passcode, card_number).
func TestAliasMatching(t *testing.T) {
	cases := []struct {
		at        AuthType
		entry     map[string]string // canonical (as stored in a set)
		submitted map[string]string // template field names
		want      bool
	}{
		{
			AuthHotel,
			map[string]string{"last_name": "Smith", "room_number": "227"},
			map[string]string{"surname": "smith", "stateroom": "227"},
			true,
		},
		{
			AuthVoucher,
			map[string]string{"code": "AB12-CD34"},
			map[string]string{"voucher": "ab12-cd34"},
			true,
		},
		{
			AuthUserPassword,
			map[string]string{"username": "jsmith", "password": "S3cret!!"},
			map[string]string{"account": "jsmith", "passcode": "S3cret!!"},
			true,
		},
		{
			AuthMembership,
			map[string]string{"member_id": "M1234567", "pin": "4242"},
			map[string]string{"card_number": "m1234567", "pin": "4242"},
			true,
		},
		// wrong password (exact match required for secrets)
		{
			AuthUserPassword,
			map[string]string{"username": "jsmith", "password": "S3cret!!"},
			map[string]string{"account": "jsmith", "passcode": "s3cret!!"},
			false,
		},
	}
	for i, c := range cases {
		if got := MatchEntry(c.at, c.submitted, c.entry); got != c.want {
			t.Errorf("case %d (%s): MatchEntry=%v, want %v", i, c.at, got, c.want)
		}
	}
}

// TestCanonicalField maps known aliases back to canonical credential fields.
func TestCanonicalField(t *testing.T) {
	for alias, want := range map[string]string{
		"stateroom": "room_number", "room": "room_number",
		"voucher": "code", "access_code": "code", "ticket_number": "code",
		"account": "username", "passcode": "password",
		"card_number": "member_id", "surname": "last_name",
		"something_else": "something_else",
	} {
		if got := CanonicalField(alias); got != want {
			t.Errorf("CanonicalField(%q)=%q, want %q", alias, got, want)
		}
	}
}
