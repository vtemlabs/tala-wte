// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

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
	return execCommand("ldapwhoami", "-x", "-H", ldapHost, "-D", dn, "-w", password).Run() == nil
}
