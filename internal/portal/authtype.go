// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package portal

import (
	"fmt"
	"math/rand"
	"strings"
)

// AuthType identifies how a captive portal authenticates and what it collects.
// A portal (built-in or user-supplied) conforms to exactly one of these.
type AuthType string

const (
	AuthClickThrough  AuthType = "click_through"     // accept terms, no credentials
	AuthEmail         AuthType = "email"             // collect an email, no validation
	AuthInfoForm      AuthType = "info_form"         // collect guest details, no validation
	AuthUserPassword  AuthType = "username_password" // username + password, validated
	AuthEmailPassword AuthType = "email_password"    // email + password, validated
	AuthHotel         AuthType = "hotel"             // last name + room number, validated
	AuthVoucher       AuthType = "voucher"           // single access code, validated
	AuthMembership    AuthType = "membership"        // member id + PIN, validated
)

// Field describes one input a portal of a given AuthType collects.
type Field struct {
	Name  string `json:"name"`  // form field name
	Label string `json:"label"` // human label
	Kind  string `json:"kind"`  // text, email, password, tel, number, checkbox
	Key   bool   `json:"key"`   // part of the credential identity (used to validate/match)
}

// AuthTypeSpec fully describes an auth type: its label, the fields it collects,
// and whether submissions are validated against a credential set.
type AuthTypeSpec struct {
	Type      AuthType `json:"type"`
	Label     string   `json:"label"`
	Desc      string   `json:"desc"`
	Fields    []Field  `json:"fields"`
	Validates bool     `json:"validates"`
}

var authSpecs = map[AuthType]AuthTypeSpec{
	AuthClickThrough: {AuthClickThrough, "Click-through", "Accept the terms and connect. No credentials are collected.", []Field{
		{Name: "accept", Label: "Accept terms", Kind: "checkbox"},
	}, false},
	AuthEmail: {AuthEmail, "Email capture", "Collect an email address before granting access (marketing-style splash).", []Field{
		{Name: "email", Label: "Email", Kind: "email"},
	}, false},
	AuthInfoForm: {AuthInfoForm, "Information form", "Collect guest details: name, email, phone, company.", []Field{
		{Name: "first_name", Label: "First name", Kind: "text"},
		{Name: "last_name", Label: "Last name", Kind: "text"},
		{Name: "email", Label: "Email", Kind: "email"},
		{Name: "phone", Label: "Phone", Kind: "tel"},
		{Name: "company", Label: "Company", Kind: "text"},
	}, false},
	AuthUserPassword: {AuthUserPassword, "Username & password", "Validate a username and password against the credential set (or the directory).", []Field{
		{Name: "username", Label: "Username", Kind: "text", Key: true},
		{Name: "password", Label: "Password", Kind: "password", Key: true},
	}, true},
	AuthEmailPassword: {AuthEmailPassword, "Email & password", "Validate an email and password against the credential set.", []Field{
		{Name: "email", Label: "Email", Kind: "email", Key: true},
		{Name: "password", Label: "Password", Kind: "password", Key: true},
	}, true},
	AuthHotel: {AuthHotel, "Hotel (room + last name)", "Validate a room number and guest last name, like a hotel Wi-Fi portal.", []Field{
		{Name: "last_name", Label: "Last name", Kind: "text", Key: true},
		{Name: "room_number", Label: "Room number", Kind: "text", Key: true},
	}, true},
	AuthVoucher: {AuthVoucher, "Voucher / access code", "Validate a single access code or voucher.", []Field{
		{Name: "code", Label: "Access code", Kind: "text", Key: true},
	}, true},
	AuthMembership: {AuthMembership, "Membership (ID + PIN)", "Validate a member or loyalty ID and a PIN.", []Field{
		{Name: "member_id", Label: "Member ID", Kind: "text", Key: true},
		{Name: "pin", Label: "PIN", Kind: "password", Key: true},
	}, true},
}

// authTypeOrder is the stable display order for the UI.
var authTypeOrder = []AuthType{
	AuthClickThrough, AuthEmail, AuthInfoForm,
	AuthUserPassword, AuthEmailPassword, AuthHotel, AuthVoucher, AuthMembership,
}

// Spec returns the spec for an auth type. Unknown types resolve to click-through.
func Spec(t AuthType) AuthTypeSpec {
	if s, ok := authSpecs[t]; ok {
		return s
	}
	return authSpecs[AuthClickThrough]
}

// AllSpecs returns every auth-type spec in display order.
func AllSpecs() []AuthTypeSpec {
	out := make([]AuthTypeSpec, 0, len(authTypeOrder))
	for _, t := range authTypeOrder {
		out = append(out, authSpecs[t])
	}
	return out
}

// KeyFields returns the names of the fields that identify a credential for a type.
func (s AuthTypeSpec) KeyFields() []string {
	var keys []string
	for _, f := range s.Fields {
		if f.Key {
			keys = append(keys, f.Name)
		}
	}
	return keys
}

// MatchEntry reports whether a submitted form satisfies a credential-set entry:
// every key field must match (case-insensitive for non-secret fields, exact for
// password/pin). Empty submitted key fields never match.
func MatchEntry(t AuthType, submitted, entry map[string]string) bool {
	keys := Spec(t).KeyFields()
	if len(keys) == 0 {
		return false
	}
	for _, k := range keys {
		got, want := strings.TrimSpace(submitted[k]), strings.TrimSpace(entry[k])
		if got == "" {
			return false
		}
		if k == "password" || k == "pin" {
			if got != want {
				return false
			}
		} else if !strings.EqualFold(got, want) {
			return false
		}
	}
	return true
}

// GenerateEntry produces one believable, validatable credential entry for a type.
// i seeds variation so a batch yields distinct entries. The returned map covers
// the type's key fields (and a display name where useful).
func GenerateEntry(t AuthType, i int) map[string]string {
	switch t {
	case AuthHotel:
		return map[string]string{
			"last_name":   credLastNames[i%len(credLastNames)],
			"room_number": fmt.Sprintf("%d%02d", 1+rand.Intn(8), 1+rand.Intn(40)),
		}
	case AuthVoucher:
		return map[string]string{"code": voucherCode()}
	case AuthMembership:
		return map[string]string{
			"member_id": fmt.Sprintf("M%07d", 1000000+rand.Intn(8999999)),
			"pin":       fmt.Sprintf("%04d", rand.Intn(10000)),
		}
	case AuthUserPassword:
		ln := credLastNames[i%len(credLastNames)]
		fn := credFirstNames[i%len(credFirstNames)]
		return map[string]string{
			"username": strings.ToLower(string(fn[0]) + ln),
			"password": credPassword(),
		}
	case AuthEmailPassword:
		ln := credLastNames[i%len(credLastNames)]
		fn := credFirstNames[i%len(credFirstNames)]
		return map[string]string{
			"email":    strings.ToLower(string(fn[0]) + ln + "@example.com"),
			"password": credPassword(),
		}
	default:
		return map[string]string{}
	}
}

func voucherCode() string {
	const a = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = a[rand.Intn(len(a))]
		if i == 3 {
			b[i] = '-'
		}
	}
	return string(b)
}

func credPassword() string {
	const lower = "abcdefghijkmnpqrstuvwxyz"
	const up = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	const dig = "23456789"
	const sym = "!@#$%&*"
	pick := func(s string) byte { return s[rand.Intn(len(s))] }
	b := []byte{pick(up), pick(lower), pick(lower), pick(lower), pick(dig), pick(dig), pick(sym), pick(lower), pick(lower)}
	return string(b)
}

var credFirstNames = []string{
	"James", "Mary", "Robert", "Patricia", "John", "Jennifer", "Michael", "Linda",
	"David", "Elizabeth", "William", "Barbara", "Richard", "Susan", "Joseph", "Jessica",
	"Thomas", "Sarah", "Chris", "Karen", "Daniel", "Nancy", "Matthew", "Lisa",
}

var credLastNames = []string{
	"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis",
	"Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez", "Wilson", "Anderson",
	"Thomas", "Taylor", "Moore", "Jackson", "Martin", "Lee", "Perez", "Thompson", "White",
}
