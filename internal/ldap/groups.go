// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

package ldap

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/pocketbase/pocketbase"

	"github.com/vtemlabs/tala-wte/internal/api"
)

// Group represents an LDAP groupOfNames entry.
type Group struct {
	CN      string   `json:"cn"`
	Members []string `json:"members"`
	DN      string   `json:"dn"`
}

// ListGroupsHandler returns all groups in ou=Groups.
func ListGroupsHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		entries, err := ldapsearch("(objectClass=groupOfNames)", "ou=Groups,"+defaultBaseDN)
		if err != nil {
			api.WriteErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		var groups []Group
		for _, e := range entries {
			groups = append(groups, entryToGroup(e))
		}
		api.WriteJSON(w, map[string]any{"groups": groups})
	}
}

// CreateGroupHandler creates a new groupOfNames.
func CreateGroupHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			CN          string   `json:"cn"`
			InitMembers []string `json:"members"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CN == "" {
			api.WriteErr(w, http.StatusBadRequest, "cn required")
			return
		}
		if err := validateDNComponent(req.CN, "cn"); err != nil {
			api.WriteErr(w, http.StatusBadRequest, err.Error())
			return
		}
		dn := fmt.Sprintf("cn=%s,ou=Groups,%s", req.CN, defaultBaseDN)
		ldif := fmt.Sprintf("dn: %s\nobjectClass: top\nobjectClass: groupOfNames\n%s\n", dn, ldifAttr("cn", req.CN))
		for _, m := range req.InitMembers {
			if err := validateDNComponent(m, "member uid"); err != nil {
				api.WriteErr(w, http.StatusBadRequest, err.Error())
				return
			}
			ldif += fmt.Sprintf("member: uid=%s,ou=Users,%s\n", m, defaultBaseDN)
		}
		if len(req.InitMembers) == 0 {
			// groupOfNames requires at least one member per schema
			ldif += fmt.Sprintf("member: cn=admin,%s\n", defaultBaseDN)
		}
		if err := ldapadd(ldif); err != nil {
			api.WriteErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		api.WriteJSON(w, map[string]any{"status": "created", "cn": req.CN})
	}
}

// DeleteGroupHandler removes a group.
func DeleteGroupHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		cn := r.PathValue("cn")
		if err := validateDNComponent(cn, "cn"); err != nil {
			api.WriteErr(w, http.StatusBadRequest, err.Error())
			return
		}
		dn := fmt.Sprintf("cn=%s,ou=Groups,%s", cn, defaultBaseDN)
		if err := ldapdelete(dn); err != nil {
			api.WriteErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		api.WriteJSON(w, map[string]any{"status": "deleted", "cn": cn})
	}
}

// AddMemberHandler adds a user to a group.
func AddMemberHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		cn := r.PathValue("cn")
		var req struct {
			UID string `json:"uid"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UID == "" {
			api.WriteErr(w, http.StatusBadRequest, "uid required")
			return
		}
		if err := validateDNComponent(cn, "cn"); err != nil {
			api.WriteErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := validateDNComponent(req.UID, "uid"); err != nil {
			api.WriteErr(w, http.StatusBadRequest, err.Error())
			return
		}
		groupDN := fmt.Sprintf("cn=%s,ou=Groups,%s", cn, defaultBaseDN)
		memberDN := fmt.Sprintf("uid=%s,ou=Users,%s", req.UID, defaultBaseDN)
		ldif := fmt.Sprintf("dn: %s\nchangetype: modify\nadd: member\nmember: %s\n", groupDN, memberDN)
		if err := ldapmodify(ldif); err != nil {
			api.WriteErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		api.WriteJSON(w, map[string]any{"status": "member_added", "cn": cn, "uid": req.UID})
	}
}

// RemoveMemberHandler removes a user from a group.
func RemoveMemberHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		cn := r.PathValue("cn")
		uid := r.PathValue("uid")
		if err := validateDNComponent(cn, "cn"); err != nil {
			api.WriteErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := validateDNComponent(uid, "uid"); err != nil {
			api.WriteErr(w, http.StatusBadRequest, err.Error())
			return
		}
		groupDN := fmt.Sprintf("cn=%s,ou=Groups,%s", cn, defaultBaseDN)
		memberDN := fmt.Sprintf("uid=%s,ou=Users,%s", uid, defaultBaseDN)
		ldif := fmt.Sprintf("dn: %s\nchangetype: modify\ndelete: member\nmember: %s\n", groupDN, memberDN)
		if err := ldapmodify(ldif); err != nil {
			api.WriteErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		api.WriteJSON(w, map[string]any{"status": "member_removed", "cn": cn, "uid": uid})
	}
}

func entryToGroup(entry map[string]string) Group {
	g := Group{
		CN: entry["cn"],
		DN: entry["dn"],
	}
	if members, ok := entry["member"]; ok && members != "" {
		for _, m := range strings.Split(members, "\n") {
			m = strings.TrimSpace(m)
			if m != "" {
				g.Members = append(g.Members, m)
			}
		}
	}
	return g
}

func execCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}
