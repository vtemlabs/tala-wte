// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/vtemlabs/tala-wte/internal/api"
	"github.com/vtemlabs/tala-wte/internal/client"
)

// clientMode reports whether this instance runs as a Tala WTE client (set by the
// install-client systemd unit via TALA_MODE=client) rather than an AP.
func clientMode() bool {
	return strings.EqualFold(os.Getenv("TALA_MODE"), "client")
}

var filenameSafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// clientConfigExportHandler returns a downloadable client connection profile for
// a network, so it can be imported into a Tala WTE client instance.
func clientConfigExportHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		rec, err := app.FindRecordById("networks", r.PathValue("id"))
		if err != nil {
			api.WriteErr(w, http.StatusNotFound, "network not found")
			return
		}
		cfg := client.Config{
			SSID:       rec.GetString("ssid"),
			Protocol:   rec.GetString("protocol"),
			Passphrase: rec.GetString("passphrase"),
			Band:       rec.GetString("band"),
			Channel:    rec.GetInt("channel"),
			Hidden:     rec.GetBool("hidden"),
		}
		cfg.Portal.Enabled = rec.GetBool("portal_enabled")

		name := filenameSafe.ReplaceAllString(rec.GetString("ssid"), "_")
		if name == "" {
			name = "network"
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="tala-client-%s.json"`, name))
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(cfg)
	}
}

// clientConnectHandler connects the client to an imported network profile. The
// join (associate + DHCP + portal) runs in the background; the console polls
// /api/wte/client/status for progress.
func clientConnectHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var cfg client.Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			api.WriteErr(w, http.StatusBadRequest, "invalid config json")
			return
		}
		if cfg.SSID == "" {
			api.WriteErr(w, http.StatusBadRequest, "config has no SSID")
			return
		}
		go func() { _ = client.Get().Connect(cfg) }()
		api.WriteJSON(w, map[string]any{"status": "connecting", "ssid": cfg.SSID})
	}
}

// clientStartHandler starts the selected traffic generators.
func clientStartHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var opts client.TrafficOptions
		if err := json.NewDecoder(r.Body).Decode(&opts); err != nil {
			api.WriteErr(w, http.StatusBadRequest, "invalid traffic options")
			return
		}
		if err := client.Get().StartTraffic(opts); err != nil {
			api.WriteErr(w, http.StatusConflict, err.Error())
			return
		}
		api.WriteJSON(w, client.Get().Status())
	}
}

// clientStopHandler halts traffic generation but keeps the connection up.
func clientStopHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		client.Get().StopTraffic()
		api.WriteJSON(w, client.Get().Status())
	}
}

// clientDisconnectHandler stops traffic and tears down the Wi-Fi connection.
func clientDisconnectHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		client.Get().Stop()
		api.WriteJSON(w, client.Get().Status())
	}
}

// clientStatusHandler returns the live client status for the console.
func clientStatusHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		api.WriteJSON(w, client.Get().Status())
	}
}
