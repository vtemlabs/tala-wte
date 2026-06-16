// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

package ldap

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/vtemlabs/tala-wte/internal/api"
)

// safeIdentifierPattern validates UID and CN values used in DN construction.
var safeIdentifierPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// modifiableAttributes is the allowlist of attributes that can be changed via UpdateUserHandler.
var modifiableAttributes = map[string]bool{
	"cn":        true,
	"sn":        true,
	"givenName": true,
	"mail":      true,
}

// validateDNComponent checks that a value is safe for use in LDAP DN construction.
func validateDNComponent(value, field string) error {
	if !safeIdentifierPattern.MatchString(value) {
		return fmt.Errorf("invalid %s: must contain only alphanumeric characters, dots, hyphens, and underscores", field)
	}
	return nil
}

// needsBase64Encoding checks whether an LDIF value requires base64 encoding per RFC 2849.
func needsBase64Encoding(val string) bool {
	if val == "" {
		return false
	}
	if val[0] == ' ' || val[0] == ':' || val[0] == '<' {
		return true
	}
	for i := 0; i < len(val); i++ {
		if val[i] == '\n' || val[i] == '\r' || val[i] == 0 || val[i] > 127 {
			return true
		}
	}
	return false
}

// ldifAttr formats an LDIF attribute line, using base64 encoding when needed per RFC 2849.
func ldifAttr(name, value string) string {
	if needsBase64Encoding(value) {
		return fmt.Sprintf("%s:: %s", name, base64.StdEncoding.EncodeToString([]byte(value)))
	}
	return fmt.Sprintf("%s: %s", name, value)
}

// User represents an LDAP user entry. Password is the userPassword attribute
// verbatim from slapd: plaintext, or a hash prefix like "{SSHA}" if stored hashed.
type User struct {
	UID       string   `json:"uid"`
	CN        string   `json:"cn"`
	SN        string   `json:"sn"`
	GivenName string   `json:"given_name"`
	Mail      string   `json:"mail"`
	Password  string   `json:"password,omitempty"`
	Groups    []string `json:"groups"`
	DN        string   `json:"dn"`
}

// ListUsersHandler returns all users in ou=Users.
func ListUsersHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := ldapsearch("(objectClass=inetOrgPerson)", "ou=Users,"+defaultBaseDN)
		if err != nil {
			api.WriteErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		var users []User
		for _, e := range entries {
			users = append(users, entryToUser(e))
		}
		api.WriteJSON(w, map[string]any{"users": users})
	}
}

// CreateUserHandler adds a new inetOrgPerson.
func CreateUserHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			UID       string `json:"uid"`
			CN        string `json:"cn"`
			SN        string `json:"sn"`
			GivenName string `json:"given_name"`
			Mail      string `json:"mail"`
			Password  string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			api.WriteErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		if req.UID == "" || req.CN == "" || req.Password == "" {
			api.WriteErr(w, http.StatusBadRequest, "uid, cn, password required")
			return
		}
		if err := validateDNComponent(req.UID, "uid"); err != nil {
			api.WriteErr(w, http.StatusBadRequest, err.Error())
			return
		}
		dn := fmt.Sprintf("uid=%s,ou=Users,%s", req.UID, defaultBaseDN)
		ldif := fmt.Sprintf("dn: %s\n", dn)
		ldif += "objectClass: top\nobjectClass: person\nobjectClass: organizationalPerson\nobjectClass: inetOrgPerson\n"
		ldif += ldifAttr("uid", req.UID) + "\n"
		ldif += ldifAttr("cn", req.CN) + "\n"
		ldif += ldifAttr("sn", req.SN) + "\n"
		ldif += ldifAttr("givenName", req.GivenName) + "\n"
		ldif += ldifAttr("mail", req.Mail) + "\n"
		ldif += ldifAttr("userPassword", req.Password) + "\n"

		if err := ldapadd(ldif); err != nil {
			api.WriteErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		api.WriteJSON(w, map[string]any{"status": "created", "dn": dn})
	}
}

// UpdateUserHandler modifies user attributes.
func UpdateUserHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := r.PathValue("uid")
		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			api.WriteErr(w, http.StatusBadRequest, "invalid json")
			return
		}

		for attr := range req {
			if !modifiableAttributes[attr] {
				api.WriteErr(w, http.StatusBadRequest, fmt.Sprintf("attribute %q is not modifiable", attr))
				return
			}
		}

		if err := validateDNComponent(uid, "uid"); err != nil {
			api.WriteErr(w, http.StatusBadRequest, err.Error())
			return
		}
		dn := fmt.Sprintf("uid=%s,ou=Users,%s", uid, defaultBaseDN)
		var mods strings.Builder
		mods.WriteString(fmt.Sprintf("dn: %s\nchangetype: modify\n", dn))
		for attr, val := range req {
			mods.WriteString(fmt.Sprintf("replace: %s\n%s\n-\n", attr, ldifAttr(attr, val)))
		}
		if err := ldapmodify(mods.String()); err != nil {
			api.WriteErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		api.WriteJSON(w, map[string]any{"status": "updated", "uid": uid})
	}
}

// DeleteUserHandler removes a user.
func DeleteUserHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := r.PathValue("uid")
		if err := validateDNComponent(uid, "uid"); err != nil {
			api.WriteErr(w, http.StatusBadRequest, err.Error())
			return
		}
		dn := fmt.Sprintf("uid=%s,ou=Users,%s", uid, defaultBaseDN)
		if err := ldapdelete(dn); err != nil {
			api.WriteErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		api.WriteJSON(w, map[string]any{"status": "deleted", "uid": uid})
	}
}

// SetPasswordHandler changes a user's password.
func SetPasswordHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := r.PathValue("uid")
		var req struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Password == "" {
			api.WriteErr(w, http.StatusBadRequest, "password required")
			return
		}
		if err := validateDNComponent(uid, "uid"); err != nil {
			api.WriteErr(w, http.StatusBadRequest, err.Error())
			return
		}
		dn := fmt.Sprintf("uid=%s,ou=Users,%s", uid, defaultBaseDN)
		ldif := fmt.Sprintf("dn: %s\nchangetype: modify\nreplace: userPassword\n%s\n", dn, ldifAttr("userPassword", req.Password))
		if err := ldapmodify(ldif); err != nil {
			api.WriteErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		api.WriteJSON(w, map[string]any{"status": "password_set", "uid": uid})
	}
}

// TestAuthHandler tests LDAP bind for a user.
func TestAuthHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			UID      string `json:"uid"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			api.WriteErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		if err := validateDNComponent(req.UID, "uid"); err != nil {
			api.WriteErr(w, http.StatusBadRequest, err.Error())
			return
		}
		dn := fmt.Sprintf("uid=%s,ou=Users,%s", req.UID, defaultBaseDN)
		out, err := execCommand("ldapwhoami",
			"-x", "-H", ldapHost,
			"-D", dn,
			"-w", req.Password,
		).CombinedOutput()
		if err != nil {
			api.WriteJSON(w, map[string]any{"success": false, "message": sanitize(string(out))})
			return
		}
		api.WriteJSON(w, map[string]any{"success": true, "dn": dn})
	}
}

func entryToUser(entry map[string]string) User {
	// LDAP attribute names are case-insensitive; parseLDIF lowercases them.
	return User{
		UID:       entry["uid"],
		CN:        entry["cn"],
		SN:        entry["sn"],
		GivenName: entry["givenname"],
		Mail:      entry["mail"],
		Password:  entry["userpassword"],
		DN:        entry["dn"],
	}
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, `"`, `'`)
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
