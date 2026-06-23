// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package main

// First-boot admin setup, performed entirely in the browser. No superuser is
// auto-provisioned and no credentials are printed. Both endpoints are
// unauthenticated by design (no superuser exists yet); /complete hard-rejects
// (409) once a real superuser exists, so setup is one-shot. A "real" superuser
// is any _superusers row that is NOT the PocketBase install placeholder
// (core.DefaultInstallerEmail).

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	"github.com/vtemlabs/tala-wte/internal/api"
)

// hasRealSuperuser reports whether any superuser exists other than the PocketBase install placeholder.
func hasRealSuperuser(app *pocketbase.PocketBase) (bool, error) {
	records, err := app.FindAllRecords(core.CollectionNameSuperusers)
	if err != nil {
		return false, err
	}
	for _, r := range records {
		if !strings.EqualFold(r.GetString("email"), core.DefaultInstallerEmail) {
			return true, nil
		}
	}
	return false, nil
}

// setupStatusHandler reports whether first-boot account setup is still needed.
func setupStatusHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		real, err := hasRealSuperuser(app)
		if err != nil {
			api.WriteErr(w, http.StatusInternalServerError, "status check failed")
			return
		}
		api.WriteJSON(w, map[string]any{"needs_setup": !real})
	}
}

// setupCompleteHandler creates the first real superuser from the browser wizard and returns an auth token. Hard no-op (409) once one exists.
func setupCompleteHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		real, err := hasRealSuperuser(app)
		if err != nil {
			api.WriteErr(w, http.StatusInternalServerError, "status check failed")
			return
		}
		if real {
			api.WriteErr(w, http.StatusConflict, "an admin account already exists; use the normal login")
			return
		}

		var body struct {
			Email      string `json:"email"`
			Password   string `json:"password"`
			LicenseAck bool   `json:"license_ack"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			api.WriteErr(w, http.StatusBadRequest, "invalid request body")
			return
		}
		body.Email = strings.TrimSpace(body.Email)
		if body.Email == "" || !strings.Contains(body.Email, "@") {
			api.WriteErr(w, http.StatusBadRequest, "a valid admin email is required")
			return
		}
		if len(body.Password) < 10 {
			api.WriteErr(w, http.StatusBadRequest, "password must be at least 10 characters")
			return
		}
		// Enforced server-side so the license gate cannot be bypassed by a direct API call.
		if !body.LicenseAck {
			api.WriteErr(w, http.StatusBadRequest, "the Tala WTE license must be acknowledged to continue")
			return
		}

		col, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
		if err != nil {
			api.WriteErr(w, http.StatusInternalServerError, "superusers collection missing")
			return
		}
		rec := core.NewRecord(col)
		rec.SetEmail(body.Email)
		rec.SetPassword(body.Password)
		rec.Set("verified", true)
		if err := app.Save(rec); err != nil {
			api.WriteErr(w, http.StatusInternalServerError, "could not create admin account")
			return
		}

		// Record the license acknowledgment alongside the account that made it.
		_ = saveSetting(app, "license_acknowledged_at", time.Now().UTC().Format(time.RFC3339))
		_ = saveSetting(app, "license_acknowledged_by", body.Email)

		// Remove the PocketBase install placeholder.
		if ph, perr := app.FindFirstRecordByFilter(
			core.CollectionNameSuperusers, "email = {:e}",
			map[string]any{"e": core.DefaultInstallerEmail},
		); perr == nil && ph != nil {
			_ = app.Delete(ph)
		}

		// Shape the response like authWithPassword so the browser can persist it via pb.authStore.save(token, record).
		token, err := rec.NewAuthToken()
		if err != nil {
			api.WriteErr(w, http.StatusInternalServerError, "account created - please log in")
			return
		}
		api.WriteJSON(w, map[string]any{
			"token": token,
			"record": map[string]any{
				"id":             rec.Id,
				"email":          body.Email,
				"collectionName": core.CollectionNameSuperusers,
			},
		})
	}
}
