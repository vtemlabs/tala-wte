// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

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
	UID        string `json:"uid"`
	CN         string `json:"cn"`
	Mail       string `json:"mail"`
	Password   string `json:"password"`
	Department string `json:"department"`
	Title      string `json:"title"`
}

// orgDept is a department template: the name doubles as the LDAP group cn and the
// user's ou attribute, Titles seeds realistic job titles, and Weight skews how
// many users land in it (a small company has many engineers, few executives).
type orgDept struct {
	Name   string
	Titles []string
	Weight int
}

// orgDepartments models the org chart of a typical small company, so a generated
// directory has the department, role, and access groups a trainee expects to
// enumerate in a real domain.
var orgDepartments = []orgDept{
	{"Engineering", []string{"Software Engineer", "Senior Software Engineer", "DevOps Engineer", "QA Engineer", "Engineering Manager"}, 5},
	{"Sales", []string{"Account Executive", "Sales Representative", "Regional Sales Manager", "Sales Development Rep"}, 4},
	{"Customer Support", []string{"Support Specialist", "Customer Success Manager", "Support Team Lead"}, 3},
	{"Information Technology", []string{"Systems Administrator", "Network Engineer", "Help Desk Technician", "IT Manager", "Security Analyst"}, 3},
	{"Marketing", []string{"Marketing Specialist", "Content Strategist", "Marketing Manager"}, 2},
	{"Finance", []string{"Accountant", "Financial Analyst", "Controller", "Accounts Payable Clerk"}, 2},
	{"Human Resources", []string{"HR Generalist", "Recruiter", "HR Manager"}, 2},
	{"Operations", []string{"Operations Analyst", "Operations Manager", "Logistics Coordinator"}, 2},
	{"Legal", []string{"Corporate Counsel", "Paralegal"}, 1},
	{"Executive", []string{"Chief Executive Officer", "Chief Financial Officer", "Chief Technology Officer", "Chief Operating Officer"}, 1},
}

// provisionGroup is an LDAP groupOfNames to emit, with its member uids.
type provisionGroup struct {
	CN          string
	Description string
	Members     []string
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
		"Aaron", "Adam", "Adrian", "Alan", "Albert", "Alexander", "Alexis", "Alice",
		"Amber", "Amy", "Andrea", "Angela", "Anna", "Austin", "Bradley", "Brandon",
		"Brenda", "Brittany", "Bruce", "Bryan", "Caleb", "Cameron", "Carl", "Carlos",
		"Catherine", "Charles", "Cheryl", "Christina", "Christine", "Cynthia", "Dale",
		"Dana", "Danielle", "Dennis", "Derek", "Diana", "Diane", "Douglas", "Dylan",
		"Edward", "Eric", "Erica", "Eugene", "Evelyn", "Frances", "Frank", "Gabriel",
		"Gary", "Gerald", "Gloria", "Grace", "Gregory", "Harold", "Heather", "Henry",
		"Howard", "Ian", "Isaac", "Jack", "Jacob", "Jacqueline", "Jane", "Janet",
		"Janice", "Jason", "Jean", "Jeffrey", "Jeremy", "Jesse", "Jessica", "Joan",
		"Jonathan", "Jordan", "Jose", "Joyce", "Juan", "Judith", "Julia", "Julie",
		"Justin", "Katherine", "Kathleen", "Kathryn", "Kayla", "Keith", "Kelly", "Kyle",
		"Larry", "Laura", "Lauren", "Lawrence", "Logan", "Louis", "Lucas", "Madison",
		"Marie", "Marilyn", "Mason", "Megan", "Nathan", "Nicholas", "Nicole", "Noah",
		"Olivia", "Pamela", "Patrick", "Peter", "Philip", "Rachel", "Ralph", "Raymond",
		"Rebecca", "Roger", "Roy", "Russell", "Ruth", "Ryan", "Samantha", "Samuel",
		"Sara", "Scott", "Sean", "Sharon", "Shirley", "Sophia", "Stephen", "Teresa",
		"Terry", "Theresa", "Tiffany", "Tyler", "Vincent", "Virginia", "Walter", "Wayne",
		"Wendy", "Zachary", "Priya", "Raj", "Wei", "Mei", "Hiro", "Yuki", "Omar", "Fatima",
		"Diego", "Sofia", "Mateo", "Camila", "Liam", "Emma", "Aisha", "Ahmed", "Ananya",
		"Arjun", "Chen", "Ling", "Kenji", "Sakura", "Ivan", "Natasha", "Sven", "Astrid",
	}
	randomLastNames = []string{
		"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis",
		"Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez", "Wilson", "Anderson",
		"Thomas", "Taylor", "Moore", "Jackson", "Martin", "Lee", "Perez", "Thompson",
		"White", "Harris", "Sanchez", "Clark", "Ramirez", "Lewis", "Robinson",
		"Walker", "Young", "Allen", "King", "Wright", "Scott", "Torres", "Nguyen",
		"Hill", "Flores", "Green", "Adams", "Nelson", "Baker", "Hall", "Rivera",
		"Campbell", "Mitchell", "Carter", "Roberts",
		"Bailey", "Bell", "Bennett", "Brooks", "Bryant", "Butler", "Cook", "Cooper",
		"Cox", "Coleman", "Collins", "Cruz", "Diaz", "Edwards", "Evans", "Fisher",
		"Foster", "Gomez", "Gray", "Griffin", "Hayes", "Henderson", "Howard", "Hughes",
		"Jenkins", "Kelly", "Kim", "Long", "Morales", "Morgan", "Morris", "Murphy",
		"Myers", "Ortiz", "Parker", "Patel", "Peterson", "Powell", "Price", "Reed",
		"Reyes", "Richardson", "Ross", "Sanders", "Simmons", "Stewart", "Sullivan",
		"Turner", "Ward", "Washington", "Watson", "Webb", "Wells", "West", "Wood",
		"Woods", "Bishop", "Chen", "Wang", "Singh", "Kumar", "Khan", "Ali", "Murray",
		"Hamilton", "Graham", "Crawford", "Olsen", "Hansen", "Schmidt", "Meyer",
		"Fischer", "Weber", "Romano", "Russo", "Costa", "Silva", "Santos", "Oliveira",
		"Nakamura", "Yamamoto", "Sato", "Tanaka", "Park", "Choi", "Tran", "Pham", "Vu",
		"Petrov", "Ivanov", "Novak", "Kowalski", "Andersson", "Larsen", "Mueller",
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

	for _, u := range users {
		fields := strings.Fields(u.CN)
		sn := fields[len(fields)-1]
		fmt.Fprintf(&ldif, `dn: uid=%s,ou=Users,%s
objectClass: top
objectClass: person
objectClass: organizationalPerson
objectClass: inetOrgPerson
uid: %s
cn: %s
sn: %s
displayName: %s
title: %s
ou: %s
mail: %s
userPassword: %s

`, u.UID, defaultBaseDN, u.UID, u.CN, sn, u.CN, u.Title, u.Department, u.Mail, u.Password)
	}

	groups := buildGroups(users)
	for _, g := range groups {
		fmt.Fprintf(&ldif, "dn: cn=%s,ou=Groups,%s\nobjectClass: top\nobjectClass: groupOfNames\ncn: %s\n", g.CN, defaultBaseDN, g.CN)
		if g.Description != "" {
			fmt.Fprintf(&ldif, "description: %s\n", g.Description)
		}
		for _, uid := range g.Members {
			fmt.Fprintf(&ldif, "member: uid=%s,ou=Users,%s\n", uid, defaultBaseDN)
		}
		ldif.WriteString("\n")
	}

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

	log.Printf("[ldap] Provisioning complete: %s - %d users, %d groups", req.CompanyName, len(users), len(groups))

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

	// Expand departments by weight so a weighted pick is a plain index lookup.
	deptPool := make([]orgDept, 0, 32)
	for _, d := range orgDepartments {
		for i := 0; i < d.Weight; i++ {
			deptPool = append(deptPool, d)
		}
	}

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

		dept := deptPool[randomInt(len(deptPool))]
		title := dept.Titles[randomInt(len(dept.Titles))]

		users = append(users, ProvisionUser{
			UID:        uid,
			CN:         first + " " + last,
			Mail:       uid + "@" + req.Domain,
			Password:   password,
			Department: dept.Name,
			Title:      title,
		})
	}

	return users
}

// buildGroups derives a realistic set of LDAP groups from the provisioned users:
// the catch-all Domain Users, one group per populated department, privilege
// groups (Domain Admins and IT-derived operator groups), access groups (VPN and
// Remote Desktop), and the Wi-Fi groups the RADIUS path expects. Empty groups are
// never emitted, since groupOfNames requires at least one member.
func buildGroups(users []ProvisionUser) []provisionGroup {
	if len(users) == 0 {
		return nil
	}

	allUIDs := make([]string, len(users))
	byDept := map[string][]string{}
	var itUsers, execUsers []string
	for i, u := range users {
		allUIDs[i] = u.UID
		byDept[u.Department] = append(byDept[u.Department], u.UID)
		switch u.Department {
		case "Information Technology":
			itUsers = append(itUsers, u.UID)
		case "Executive":
			execUsers = append(execUsers, u.UID)
		}
	}

	// Domain Admins is small and privileged: IT staff plus executives, capped, and
	// never empty (a directory always has at least one admin).
	admins := capN(dedup(append(append([]string{}, itUsers...), execUsers...)), 3)
	if len(admins) == 0 {
		admins = []string{users[0].UID}
	}

	groups := []provisionGroup{
		{"Domain Users", "All domain user accounts", allUIDs},
		{"Domain Admins", "Domain administrators (full control)", admins},
	}

	// One group per populated department, in the org-chart order above.
	for _, d := range orgDepartments {
		if m := byDept[d.Name]; len(m) > 0 {
			groups = append(groups, provisionGroup{d.Name, d.Name + " department", m})
		}
	}

	// Privilege/operator groups derived from IT and the executive team.
	if len(itUsers) > 0 {
		groups = append(groups,
			provisionGroup{"Help Desk", "Tier-1 support operators", itUsers},
			provisionGroup{"Backup Operators", "May back up and restore all files", capN(itUsers, 2)},
			provisionGroup{"File Server Admins", "Full control of file shares", itUsers},
		)
	}
	if len(execUsers) > 0 {
		groups = append(groups, provisionGroup{"Executives", "Executive leadership team", execUsers})
	}

	// Access groups: realistic subsets of the company.
	if vpn := randomSubset(allUIDs, 55); len(vpn) > 0 {
		groups = append(groups, provisionGroup{"VPN Users", "Permitted remote VPN access", vpn})
	}
	rdp := dedup(append(append([]string{}, itUsers...), randomSubset(byDept["Engineering"], 60)...))
	if len(rdp) > 0 {
		groups = append(groups, provisionGroup{"Remote Desktop Users", "Permitted RDP access to servers", rdp})
	}

	// Wi-Fi groups consumed by the RADIUS path: everyone may join, admins manage.
	groups = append(groups,
		provisionGroup{"wifi-users", "Wi-Fi network access", allUIDs},
		provisionGroup{"wifi-admins", "Wi-Fi administration", admins},
	)
	return groups
}

func dedup(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func capN(in []string, n int) []string {
	if len(in) <= n {
		return in
	}
	return in[:n]
}

// randomSubset returns each element with the given percent probability, but never
// an empty set when the input is non-empty (groupOfNames needs a member).
func randomSubset(in []string, percent int) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if randomInt(100) < percent {
			out = append(out, s)
		}
	}
	if len(out) == 0 && len(in) > 0 {
		out = append(out, in[randomInt(len(in))])
	}
	return out
}
