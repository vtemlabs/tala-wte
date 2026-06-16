// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

package ldap

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pocketbase/pocketbase"

	"github.com/vtemlabs/tala-wte/internal/api"
)

// ProvisionRequest defines a directory provisioning template.
type ProvisionRequest struct {
	CompanyName     string `json:"company_name"`
	Domain          string `json:"domain"`
	UserCount       int    `json:"user_count"`       // 1-50
	RandomPasswords bool   `json:"random_passwords"` // true = strong random for all; false (default) = realistic mix
}

// ProvisionResponse returns the created directory details.
type ProvisionResponse struct {
	Status      string          `json:"status"`
	CompanyName string          `json:"company_name"`
	Domain      string          `json:"domain"`
	Users       []ProvisionUser `json:"users"`
}

// ProvisionUser is a user created during provisioning.
type ProvisionUser struct {
	UID      string `json:"uid"`
	CN       string `json:"cn"`
	Mail     string `json:"mail"`
	Password string `json:"password"`
}

var (
	randomFirstNames = []string{
		"James", "Mary", "Robert", "Patricia", "John", "Jennifer", "Michael", "Linda",
		"David", "Elizabeth", "William", "Barbara", "Richard", "Susan", "Joseph",
		"Margaret", "Thomas", "Dorothy", "Christopher", "Lisa", "Daniel", "Nancy",
		"Matthew", "Karen", "Anthony", "Betty", "Mark", "Helen", "Donald", "Sandra",
		"Steven", "Ashley", "Paul", "Kimberly", "Andrew", "Emily", "Joshua", "Donna",
		"Kenneth", "Michelle", "Kevin", "Carol", "Brian", "Amanda", "George", "Melissa",
		"Timothy", "Deborah", "Ronald", "Stephanie",
	}
	randomLastNames = []string{
		"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis",
		"Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez", "Wilson", "Anderson",
		"Thomas", "Taylor", "Moore", "Jackson", "Martin", "Lee", "Perez", "Thompson",
		"White", "Harris", "Sanchez", "Clark", "Ramirez", "Lewis", "Robinson",
		"Walker", "Young", "Allen", "King", "Wright", "Scott", "Torres", "Nguyen",
		"Hill", "Flores", "Green", "Adams", "Nelson", "Baker", "Hall", "Rivera",
		"Campbell", "Mitchell", "Carter", "Roberts",
	}
	randomCompanies = []string{
		"Meridian Systems", "Vanguard Industries", "Apex Dynamics", "Summit Technologies",
		"Cascade Networks", "Horizon Digital", "Atlas Security Group", "Pinnacle Defense",
		"Ironclad Solutions", "Sentinel Corp", "Obsidian Labs", "Ridgeline Ops",
		"Blackwater Analytics", "Citadel Infosec", "Fortress Digital", "Paladin Group",
	}
	randomDomains = []string{
		"meridian-sys.local", "vanguard-ind.local", "apex-dyn.local", "summit-tech.local",
		"cascade-net.local", "horizon-dig.local", "atlas-sec.local", "pinnacle-def.local",
		"ironclad-sol.local", "sentinel-corp.local", "obsidian-labs.local", "ridgeline-ops.local",
		"blackwater-ai.local", "citadel-is.local", "fortress-dig.local", "paladin-grp.local",
	}
)

func randomInt(max int) int {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max)))
	return int(n.Int64())
}

func randomPassword(length int) string {
	const charset = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#$"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[randomInt(len(charset))]
	}
	return string(b)
}

// ProvisionHandler wipes the LDAP directory and provisions a new one.
func ProvisionHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ProvisionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			api.WriteErr(w, http.StatusBadRequest, "invalid json")
			return
		}

		if req.CompanyName == "" {
			req.CompanyName = "ACME Corp"
		}
		if req.Domain == "" {
			req.Domain = "acmecorp.local"
		}
		if req.UserCount <= 0 {
			req.UserCount = 15
		}
		if req.UserCount > 50 {
			req.UserCount = 50
		}

		resp, err := provisionDirectory(req)
		if err != nil {
			api.WriteErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		api.WriteJSON(w, resp)
	}
}

// RandomProvisionHandler generates a random-company directory using the realistic
// password mix, so the trainee has something to crack.
func RandomProvisionHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		idx := randomInt(len(randomCompanies))
		req := ProvisionRequest{
			CompanyName:     randomCompanies[idx],
			Domain:          randomDomains[idx],
			UserCount:       10 + randomInt(11), // 10-20 users
			RandomPasswords: false,              // mixed distribution by default
		}

		resp, err := provisionDirectory(req)
		if err != nil {
			api.WriteErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		api.WriteJSON(w, resp)
	}
}

// UserCount returns the number of inetOrgPerson entries under ou=Users, or
// (0, nil) when slapd is not running.
func UserCount() (int, error) {
	if !IsRunning() {
		return 0, nil
	}
	entries, err := ldapsearch("(objectClass=inetOrgPerson)", "ou=Users,"+defaultBaseDN)
	if err != nil {
		return 0, err
	}
	return len(entries), nil
}

// HasUsers is a convenience wrapper that returns true iff at least one user exists.
func HasUsers() bool {
	n, err := UserCount()
	return err == nil && n > 0
}

// ProvisionDefault wipes the directory and reloads the ACME Corp baseline (15
// users, mixed passwords, two groups) for one-click preflight auto-fix.
func ProvisionDefault() (*ProvisionResponse, error) {
	return provisionDirectory(ProvisionRequest{
		CompanyName:     "ACME Corp",
		Domain:          "acmecorp.local",
		UserCount:       15,
		RandomPasswords: false,
	})
}

// stripLDIFControls removes control characters (newlines, CR, NUL, other C0 +
// DEL) so a value interpolated into an LDIF line cannot add lines/attributes.
func stripLDIFControls(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

func provisionDirectory(req ProvisionRequest) (*ProvisionResponse, error) {
	// Strip control characters so the company name/domain cannot inject extra
	// LDIF lines; the provisioning path interpolates them directly (unlike the
	// CRUD path, which escapes via ldifAttr).
	req.CompanyName = stripLDIFControls(req.CompanyName)
	req.Domain = stripLDIFControls(req.Domain)
	log.Printf("[ldap] Provisioning new directory: company=%q domain=%q users=%d", req.CompanyName, req.Domain, req.UserCount)

	Stop()

	dbDir := filepath.Join(ldapDataDir, "db")
	if err := os.RemoveAll(dbDir); err != nil {
		return nil, fmt.Errorf("remove ldap db: %w", err)
	}
	if err := os.MkdirAll(dbDir, 0o750); err != nil {
		return nil, fmt.Errorf("mkdir ldap db: %w", err)
	}

	_ = os.Remove(ldapConfFile) // best-effort: a stale config is overwritten below regardless

	if err := writeDefaultConfig(); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	users := generateUsers(req)

	var ldif bytes.Buffer

	fmt.Fprintf(&ldif, `dn: %s
objectClass: top
objectClass: dcObject
objectClass: organization
o: %s
dc: tala

dn: cn=admin,%s
objectClass: simpleSecurityObject
objectClass: organizationalRole
cn: admin
description: %s LDAP Administrator
userPassword: %s

dn: ou=Users,%s
objectClass: top
objectClass: organizationalUnit
ou: Users

dn: ou=Groups,%s
objectClass: top
objectClass: organizationalUnit
ou: Groups

`, defaultBaseDN, req.CompanyName, defaultBaseDN, req.CompanyName, AdminPassword(), defaultBaseDN, defaultBaseDN)

	memberLines := ""
	for _, u := range users {
		fmt.Fprintf(&ldif, `dn: uid=%s,ou=Users,%s
objectClass: top
objectClass: person
objectClass: organizationalPerson
objectClass: inetOrgPerson
uid: %s
cn: %s
sn: %s
mail: %s
userPassword: %s

`, u.UID, defaultBaseDN, u.UID, u.CN, strings.Fields(u.CN)[len(strings.Fields(u.CN))-1], u.Mail, u.Password)
		memberLines += fmt.Sprintf("member: uid=%s,ou=Users,%s\n", u.UID, defaultBaseDN)
	}

	fmt.Fprintf(&ldif, `dn: cn=wifi-users,ou=Groups,%s
objectClass: top
objectClass: groupOfNames
cn: wifi-users
%s
dn: cn=wifi-admins,ou=Groups,%s
objectClass: top
objectClass: groupOfNames
cn: wifi-admins
member: uid=%s,ou=Users,%s
`, defaultBaseDN, memberLines, defaultBaseDN, users[0].UID, defaultBaseDN)

	bootstrapFile := filepath.Join(ldapDataDir, "bootstrap.ldif")
	if err := os.WriteFile(bootstrapFile, ldif.Bytes(), 0o640); err != nil {
		return nil, fmt.Errorf("write ldif: %w", err)
	}

	if out, err := execCommand("slapadd", "-f", ldapConfFile, "-l", bootstrapFile).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("slapadd: %s: %w", out, err)
	}

	if err := Start(); err != nil {
		return nil, fmt.Errorf("restart slapd: %w", err)
	}

	log.Printf("[ldap] Provisioning complete: %s - %d users, 2 groups", req.CompanyName, len(users))

	return &ProvisionResponse{
		Status:      "provisioned",
		CompanyName: req.CompanyName,
		Domain:      req.Domain,
		Users:       users,
	}, nil
}

func generateUsers(req ProvisionRequest) []ProvisionUser {
	used := make(map[string]bool)
	users := make([]ProvisionUser, 0, req.UserCount)

	for i := 0; i < req.UserCount; i++ {
		var first, last, uid string
		for {
			first = randomFirstNames[randomInt(len(randomFirstNames))]
			last = randomLastNames[randomInt(len(randomLastNames))]
			uid = strings.ToLower(string(first[0]) + last)
			if !used[uid] {
				used[uid] = true
				break
			}
		}

		// RandomPasswords=true gives every user a strong random password; false
		// (default) gives the realistic mix needed for training.
		var password string
		if req.RandomPasswords {
			password = randomPassword(12)
		} else {
			password = generateMixedPassword(first, last, req.CompanyName)
		}

		users = append(users, ProvisionUser{
			UID:      uid,
			CN:       first + " " + last,
			Mail:     uid + "@" + req.Domain,
			Password: password,
		})
	}

	return users
}
