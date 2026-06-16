// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

package main

// Version reporting and the one-click self-update endpoint. Both are
// superuser-only (registered with wrapAuth in main.go). versionHandler is cheap
// and called on UI load; updateHandler downloads, verifies, swaps the binary,
// and schedules a service restart.

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/vtemlabs/tala-wte/internal/api"
	"github.com/vtemlabs/tala-wte/internal/updater"
)

// versionHandler reports the running version and, when GitHub is reachable,
// whether a newer release exists. A failed check is non-fatal: the current
// version is still returned with the error noted, so the UI degrades gracefully.
func versionHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		st, err := updater.CheckLatest(ctx)
		if err != nil {
			api.WriteErr(w, http.StatusInternalServerError, "version check failed: "+err.Error())
			return
		}
		api.WriteJSON(w, st)
	}
}

// updateHandler applies the latest release in place, then schedules a restart.
// The download + checksum verification happen synchronously so a failure is
// reported to the caller; the actual restart is deferred a couple of seconds by
// the updater so this response reaches the browser before the service bounces.
func updateHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Give the download a generous window independent of the request; the
		// binary plus checksum fetch can take a while on a slow uplink.
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
		defer cancel()

		installed, err := updater.Apply(ctx)
		if err != nil {
			log.Printf("[update] apply failed: %v", err)
			api.WriteErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		log.Printf("[update] installed version %s; service restart scheduled", installed)
		api.WriteJSON(w, map[string]any{
			"status":     "updating",
			"version":    installed,
			"restarting": true,
			"message":    "Update " + installed + " installed. The service is restarting; the page will reconnect shortly.",
		})
	}
}
