// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package ldap

import "strings"

// Authenticate verifies a username/password against the embedded directory via
// an LDAP bind as uid=<user>,ou=Users,<baseDN>, reducing an email to its local
// part (jsmith@acmecorp.local -> jsmith). Returns true only on a successful bind.
func Authenticate(username, password string) bool {
	username = strings.TrimSpace(username)
	if i := strings.IndexByte(username, '@'); i > 0 {
		username = username[:i]
	}
	if username == "" || password == "" {
		return false
	}
	if err := validateDNComponent(username, "uid"); err != nil {
		return false
	}
	dn := "uid=" + username + ",ou=Users," + defaultBaseDN
	_, err := withPasswordFile(password, func(pw string) ([]byte, error) {
		return execCommand("ldapwhoami", "-x", "-H", ldapHost, "-D", dn, "-y", pw).CombinedOutput()
	})
	return err == nil
}
