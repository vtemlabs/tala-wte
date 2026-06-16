// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

package ldap

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"text/template"
)

// hashSSHA produces an SSHA hash of the password for use as rootpw in slapd.conf.
func hashSSHA(password string) string {
	salt := make([]byte, 8)
	if _, err := rand.Read(salt); err != nil {
		return password // fall back to plaintext if randomness fails
	}
	h := sha1.New()
	h.Write([]byte(password))
	h.Write(salt)
	hash := h.Sum(nil)
	return "{SSHA}" + base64.StdEncoding.EncodeToString(append(hash, salt...))
}

func writeDefaultConfig() error {
	tmpl := template.Must(template.New("slapd").Parse(slapdConfTemplate))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]string{
		"DataDir":       ldapDataDir,
		"AdminPassword": hashSSHA(AdminPassword()),
	}); err != nil {
		return err
	}
	return os.WriteFile(ldapConfFile, buf.Bytes(), 0o640)
}

func bootstrapDirectory() error {
	var ldif bytes.Buffer

	baseLdif := fmt.Sprintf(`dn: %s
objectClass: top
objectClass: dcObject
objectClass: organization
o: ACME Corp
dc: tala

dn: cn=admin,%s
objectClass: simpleSecurityObject
objectClass: organizationalRole
cn: admin
description: ACME Corp LDAP Administrator
userPassword: %s

dn: ou=Users,%s
objectClass: top
objectClass: organizationalUnit
ou: Users

dn: ou=Groups,%s
objectClass: top
objectClass: organizationalUnit
ou: Groups

`, defaultBaseDN, defaultBaseDN, AdminPassword(), defaultBaseDN, defaultBaseDN)
	ldif.WriteString(baseLdif)

	firstNames := []string{"James", "Mary", "Robert", "Patricia", "John", "Jennifer", "Michael", "Linda", "David", "Elizabeth", "William", "Barbara", "Richard", "Susan", "Joseph"}
	lastNames := []string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis", "Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez", "Wilson", "Anderson"}

	memberLines := ""

	for i := 0; i < 15; i++ {
		first := firstNames[i]
		last := lastNames[i]
		uid := fmt.Sprintf("%c%s", first[0], last) // e.g. jsmith
		uid = string(bytes.ToLower([]byte(uid)))

		// Realistic mix, not a universal Password1!; this is what the operator's
		// first enterprise SSID will be tested against.
		password := generateMixedPassword(first, last, "ACME Corp")

		userLdif := fmt.Sprintf(`dn: uid=%s,ou=Users,%s
objectClass: top
objectClass: person
objectClass: organizationalPerson
objectClass: inetOrgPerson
uid: %s
cn: %s %s
sn: %s
givenName: %s
mail: %s@acmecorp.local
userPassword: %s

`, uid, defaultBaseDN, uid, first, last, last, first, uid, password)
		ldif.WriteString(userLdif)
		memberLines += fmt.Sprintf("member: uid=%s,ou=Users,%s\n", uid, defaultBaseDN)
	}

	groupsLdif := fmt.Sprintf(`dn: cn=wifi-users,ou=Groups,%s
objectClass: top
objectClass: groupOfNames
cn: wifi-users
%s
dn: cn=wifi-admins,ou=Groups,%s
objectClass: top
objectClass: groupOfNames
cn: wifi-admins
member: uid=%s,ou=Users,%s
`, defaultBaseDN, memberLines, defaultBaseDN, "jsmith", defaultBaseDN)

	ldif.WriteString(groupsLdif)

	bootstrapFile := ldapDataDir + "/bootstrap.ldif"
	if err := os.WriteFile(bootstrapFile, ldif.Bytes(), 0o640); err != nil {
		return fmt.Errorf("write bootstrap ldif: %w", err)
	}

	cmd := exec.Command("slapadd", "-f", ldapConfFile, "-l", bootstrapFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("slapadd: %s: %w", out, err)
	}
	return nil
}

var slapdConfTemplate = `# Tala WTE - OpenLDAP slapd configuration
# Generated automatically

include         /etc/ldap/schema/core.schema
include         /etc/ldap/schema/cosine.schema
include         /etc/ldap/schema/nis.schema
include         /etc/ldap/schema/inetorgperson.schema

pidfile         {{.DataDir}}/slapd.pid
argsfile        {{.DataDir}}/slapd.args

modulepath      /usr/lib/ldap
moduleload      back_mdb

database        mdb
maxsize         1073741824
suffix          "dc=tala,dc=wte"
rootdn          "cn=admin,dc=tala,dc=wte"
rootpw          {{.AdminPassword}}
directory       {{.DataDir}}/db
index           objectClass eq
index           uid         eq,sub
index           cn          eq,sub
index           mail        eq

access to attrs=userPassword
    by self     write
    by anonymous auth
    by *        none

access to *
    by self     write
    by users    read
    by anonymous auth
`
