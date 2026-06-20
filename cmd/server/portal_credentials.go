// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	"github.com/vtemlabs/tala-wte/internal/api"
	"github.com/vtemlabs/tala-wte/internal/portal"
)

// portalAuthTypesHandler returns the catalog of captive-portal auth types and the
// fields each collects, so the UI can present them and render typed forms.
func portalAuthTypesHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		api.WriteJSON(w, map[string]any{"auth_types": portal.AllSpecs()})
	}
}

// portalCredentialsGenerateHandler generates a random, validatable credential set
// for an auth type and stores it as a portal_credentials record so a portal of
// that type validates real submissions against it.
func portalCredentialsGenerateHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name     string `json:"name"`
			AuthType string `json:"auth_type"`
			Count    int    `json:"count"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			api.WriteErr(w, http.StatusBadRequest, "invalid request")
			return
		}
		at := portal.AuthType(req.AuthType)
		if !portal.Spec(at).Validates {
			api.WriteErr(w, http.StatusBadRequest, "this auth type does not use credentials")
			return
		}
		if req.Count < 1 {
			req.Count = 25
		}
		if req.Count > 1000 {
			req.Count = 1000
		}

		entries := make([]map[string]string, 0, req.Count)
		seen := map[string]bool{}
		for i := 0; len(entries) < req.Count && i < req.Count*5; i++ {
			e := portal.GenerateEntry(at, i)
			key := fmt.Sprint(e)
			if seen[key] {
				continue
			}
			seen[key] = true
			entries = append(entries, e)
		}
		data, _ := json.Marshal(entries)

		col, err := app.FindCollectionByNameOrId("portal_credentials")
		if err != nil {
			api.WriteErr(w, http.StatusInternalServerError, "credential collection unavailable")
			return
		}
		name := req.Name
		if name == "" {
			name = portal.Spec(at).Label + " set"
		}
		rec := core.NewRecord(col)
		rec.Set("name", name)
		rec.Set("auth_type", req.AuthType)
		rec.Set("entries", string(data))
		rec.Set("type", "custom")
		rec.Set("description", fmt.Sprintf("%d generated %s credentials", len(entries), portal.Spec(at).Label))
		if err := app.Save(rec); err != nil {
			api.WriteErr(w, http.StatusInternalServerError, "failed to save credential set: "+err.Error())
			return
		}
		api.WriteJSON(w, map[string]any{"id": rec.Id, "name": name, "count": len(entries)})
	}
}
