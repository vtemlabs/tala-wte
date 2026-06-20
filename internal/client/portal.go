// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. See the LICENSE file.

package client

import (
	"hash/fnv"
	"net/url"
	"os"
	"strings"

	"golang.org/x/net/html"

	"github.com/vtemlabs/tala-wte/internal/portal"
)

// portalIdentity is a coherent fake person used to fill a captive-portal form so
// the harvested credentials and PII look like a real user (an AD/corp login,
// hotel guest, ISP subscriber, etc.) instead of obvious placeholders.
type portalIdentity struct {
	First, Last, User, Email, Password string
}

// portalPeople are believable corp/AD-style identities. One is chosen per member
// (seeded by hostname) so each member harvests as a consistent, distinct person.
var portalPeople = []portalIdentity{
	{"John", "Smith", "jsmith", "jsmith@corp.local", "Summer2026!"},
	{"Maria", "Rodriguez", "mrodriguez", "mrodriguez@corp.local", "Falcons#2025"},
	{"Aisha", "Khan", "akhan", "akhan@corp.local", "Welcome123!"},
	{"Liam", "Chen", "lchen", "lchen@corp.local", "P@ssw0rd!"},
	{"Emma", "Johnson", "ejohnson", "ejohnson@corp.local", "Spring!2026"},
	{"Noah", "Williams", "nwilliams", "nwilliams@corp.local", "Falcon$2025"},
	{"Olivia", "Brown", "obrown", "obrown@corp.local", "Winter#2026"},
	{"James", "Garcia", "jgarcia", "jgarcia@corp.local", "Autumn!2024"},
	{"Sophia", "Miller", "smiller", "smiller@corp.local", "Sunset2026!"},
	{"Daniel", "Davis", "ddavis", "ddavis@corp.local", "C0mpl3x!Pwd"},
}

// portalIdentityFor returns the operator-provided portal credentials when set,
// otherwise a believable AD-style identity chosen consistently for this host.
func portalIdentityFor(pc PortalConfig) portalIdentity {
	if strings.TrimSpace(pc.Username) != "" {
		u := pc.Username
		return portalIdentity{
			First:    "Alex",
			Last:     "Doe",
			User:     u,
			Email:    emailFor(u),
			Password: orDefault(pc.Password, "Wifi-Pass-2026!"),
		}
	}
	host, _ := os.Hostname()
	h := fnv.New32a()
	_, _ = h.Write([]byte(host))
	return portalPeople[int(h.Sum32())%len(portalPeople)]
}

// buildPortalSubmission parses a captive-portal page and returns the form's
// action plus a filled value set. It fills recognized credential fields with a
// coherent identity, generates believable PII, checks accept/terms/opt-in boxes,
// and picks the first option for radios and selects, so a member can satisfy any
// of the portal templates instead of blindly posting a bare accept.
func buildPortalSubmission(pageHTML string, pc PortalConfig) (action string, values url.Values) {
	values = url.Values{}
	doc, err := html.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return "", values
	}
	form := findFirstForm(doc)
	if form == nil {
		return "", values
	}
	action = strings.TrimSpace(getNodeAttr(form, "action"))
	id := portalIdentityFor(pc)
	dataDefaults := collectDataDefaults(form)

	radioSeen := map[string]bool{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "input":
				fillInput(values, n, id, radioSeen, dataDefaults, pc.Fields)
			case "select":
				if name := getNodeAttr(n, "name"); name != "" {
					values.Set(name, firstOptionValue(n))
				}
			case "textarea":
				if name := getNodeAttr(n, "name"); name != "" && values.Get(name) == "" {
					values.Set(name, "n/a")
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(form)

	// A bare accept never hurts and satisfies accept-only portals.
	if values.Get("accept") == "" {
		values.Set("accept", "1")
	}
	return action, values
}

func fillInput(values url.Values, n *html.Node, id portalIdentity, radioSeen map[string]bool, dataDefaults, creds map[string]string) {
	name := getNodeAttr(n, "name")
	if name == "" {
		return
	}
	typ := strings.ToLower(getNodeAttr(n, "type"))
	val := getNodeAttr(n, "value")
	// A pushed credential (canonical field) wins for the matching data input, so a
	// deployed member submits a valid typed credential (hotel room+name, voucher,
	// login, etc.) instead of a guessed value.
	switch typ {
	case "checkbox", "radio", "submit", "button", "reset", "image", "file":
		// not a credential text field; fall through to the normal handling below
	default:
		if v := creds[portal.CanonicalField(name)]; v != "" {
			values.Set(name, v)
			return
		}
	}
	switch typ {
	case "hidden":
		// A JS-populated hidden field (set by clicking an option) arrives empty;
		// emulate the first option's data-<name>, else a generic non-empty flag.
		if strings.TrimSpace(val) == "" {
			if dv := dataDefaults[strings.ToLower(name)]; dv != "" {
				val = dv
			} else {
				val = "1"
			}
		}
		values.Set(name, val)
	case "checkbox":
		values.Set(name, orDefault(val, "on")) // check terms/accept/opt-in
	case "radio":
		if !radioSeen[name] {
			radioSeen[name] = true
			values.Set(name, orDefault(val, "on")) // pick the first option
		}
	case "password":
		values.Set(name, id.Password)
	case "submit", "button", "reset", "image", "file":
		// not user-entered data
	default: // text, email, tel, number, search, url, ...
		values.Set(name, portalFieldValue(name, typ, id))
	}
}

// collectDataDefaults maps a field name to the first data-<field> attribute value
// in the form. Click-to-select widgets (plan tiers, room types) carry the value a
// click would copy into a hidden field of the same name on a data-<field> attr, so
// this lets the filler emulate that selection without running JavaScript.
func collectDataDefaults(form *html.Node) map[string]string {
	out := map[string]string{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			for _, a := range n.Attr {
				if key, ok := strings.CutPrefix(strings.ToLower(a.Key), "data-"); ok {
					if strings.TrimSpace(a.Val) != "" && out[key] == "" {
						out[key] = a.Val
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(form)
	return out
}

// portalFieldValue picks a value for a named text field from the chosen identity
// so portals that harvest credentials or PII record realistic, coherent data.
func portalFieldValue(name, typ string, id portalIdentity) string {
	n := strings.ToLower(name)
	has := func(subs ...string) bool {
		for _, s := range subs {
			if strings.Contains(n, s) {
				return true
			}
		}
		return false
	}
	switch {
	case typ == "email" || has("email", "e-mail", "upn"):
		return id.Email
	case has("first"):
		return id.First
	case has("last", "surname", "family"):
		return id.Last
	case has("user", "login", "userid", "uid", "signin", "logon"):
		return id.User
	case has("rewards", "member", "loyalty", "account", "subscriber"):
		return id.User
	case has("room"):
		return "1204"
	case has("zip", "postal"):
		return "10001"
	case has("phone", "tel", "mobile", "cell"):
		return "5551234567"
	case has("pin", "code"):
		return "1234"
	case has("name", "guest"):
		return id.First + " " + id.Last
	default:
		return id.User
	}
}

func emailFor(user string) string {
	user = strings.TrimSpace(user)
	if user == "" {
		return ""
	}
	if strings.Contains(user, "@") {
		return user
	}
	return user + "@corp.local"
}

func orDefault(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}

func findFirstForm(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.Data == "form" {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if f := findFirstForm(c); f != nil {
			return f
		}
	}
	return nil
}

func getNodeAttr(n *html.Node, key string) string {
	v, _ := getNodeAttrOK(n, key)
	return v
}

func getNodeAttrOK(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val, true
		}
	}
	return "", false
}

// firstOptionValue returns the first real option's value, skipping a placeholder
// option that declares an explicit empty value (e.g. <option value="">Select...).
// An option with no value attribute falls back to its visible text.
func firstOptionValue(sel *html.Node) string {
	var result string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if result != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "option" {
			val, hasVal := getNodeAttrOK(n, "value")
			if !hasVal && n.FirstChild != nil {
				val = strings.TrimSpace(n.FirstChild.Data)
			}
			if strings.TrimSpace(val) != "" {
				result = val
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(sel)
	return result
}
