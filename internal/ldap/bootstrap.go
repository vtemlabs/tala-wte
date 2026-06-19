// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package ldap

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
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

// hashRootpw hashes password for slapd using SHA-512 crypt ({CRYPT}$6$), which
// slapd verifies natively through crypt(3). It shells to slappasswd (shipped
// with slapd) and falls back to salted SSHA only if slappasswd is missing or
// fails, so password hashing never blocks LDAP bring-up.
func hashRootpw(password string) string {
	out, err := withPasswordFile(password, func(p string) ([]byte, error) {
		return exec.Command("slappasswd", "-h", "{CRYPT}", "-c", "$6$%.16s", "-T", p).Output()
	})
	if err == nil {
		if h := strings.TrimSpace(string(out)); strings.HasPrefix(h, "{CRYPT}$6$") {
			return h
		}
	}
	return hashSSHA(password)
}

func writeDefaultConfig() error {
	tmpl := template.Must(template.New("slapd").Parse(slapdConfTemplate))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]string{
		"DataDir":       ldapDataDir,
		"AdminPassword": hashRootpw(AdminPassword()),
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

`, defaultBaseDN, defaultBaseDN, hashRootpw(AdminPassword()), defaultBaseDN, defaultBaseDN)
	ldif.WriteString(baseLdif)

	firstNames := []string{"James", "Mary", "Robert", "Patricia", "John", "Jennifer", "Michael", "Linda", "David", "Elizabeth", "William", "Barbara", "Richard", "Susan", "Joseph"}
	lastNames := []string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis", "Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez", "Wilson", "Anderson"}

	// Deterministic baseline so a known account (jsmith) always exists for the
	// operator's first enterprise SSID test; departments round-robin so every
	// group is represented. The randomized provisioner shares buildGroups below.
	users := make([]ProvisionUser, 0, len(firstNames))
	for i := range firstNames {
		first := firstNames[i]
		last := lastNames[i]
		uid := strings.ToLower(fmt.Sprintf("%c%s", first[0], last)) // e.g. jsmith
		dept := orgDepartments[i%len(orgDepartments)]
		title := dept.Titles[i%len(dept.Titles)]
		users = append(users, ProvisionUser{
			UID:        uid,
			CN:         first + " " + last,
			Mail:       uid + "@acmecorp.local",
			Password:   generateMixedPassword(first, last, "ACME Corp"),
			Department: dept.Name,
			Title:      title,
		})
	}

	for i, u := range users {
		fmt.Fprintf(&ldif, `dn: uid=%s,ou=Users,%s
objectClass: top
objectClass: person
objectClass: organizationalPerson
objectClass: inetOrgPerson
uid: %s
cn: %s
sn: %s
givenName: %s
displayName: %s
title: %s
ou: %s
mail: %s
userPassword: %s

`, u.UID, defaultBaseDN, u.UID, u.CN, lastNames[i], firstNames[i], u.CN, u.Title, u.Department, u.Mail, u.Password)
	}

	for _, g := range buildGroups(users) {
		fmt.Fprintf(&ldif, "dn: cn=%s,ou=Groups,%s\nobjectClass: top\nobjectClass: groupOfNames\ncn: %s\n", g.CN, defaultBaseDN, g.CN)
		if g.Description != "" {
			fmt.Fprintf(&ldif, "description: %s\n", g.Description)
		}
		for _, uid := range g.Members {
			fmt.Fprintf(&ldif, "member: uid=%s,ou=Users,%s\n", uid, defaultBaseDN)
		}
		ldif.WriteString("\n")
	}

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
