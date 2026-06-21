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
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase"

	"github.com/vtemlabs/tala-wte/internal/api"
	"github.com/vtemlabs/tala-wte/internal/client"
	"github.com/vtemlabs/tala-wte/internal/deps"
)

// modeOverrideFile persists the AP/client role chosen via the in-app swap. It
// takes precedence over the install-time TALA_MODE env so the choice survives a
// restart, letting one binary switch roles without reinstalling.
var modeOverrideFile = installDataDir + "/mode"

// clientMode reports whether this instance runs as a Tala WTE client rather than
// an AP. The persisted swap file wins; otherwise it falls back to the install-time
// TALA_MODE env (set by the install-client systemd unit).
func clientMode() bool {
	if b, err := os.ReadFile(modeOverrideFile); err == nil {
		switch strings.TrimSpace(string(b)) {
		case "client":
			return true
		case "ap", "server":
			return false
		}
	}
	return strings.EqualFold(os.Getenv("TALA_MODE"), "client")
}

// clientAutoconnectFile persists the last client connection so the agent
// reconnects to it on its own after a reboot or crash, without the pack leader
// re-driving it. Cleared on a deliberate disconnect.
var clientAutoconnectFile = installDataDir + "/client-autoconnect.json"

func saveAutoconnect(cfg client.Config) {
	if b, err := json.Marshal(cfg); err == nil {
		_ = os.WriteFile(clientAutoconnectFile, b, 0o600)
	}
}

func clearAutoconnect() { _ = os.Remove(clientAutoconnectFile) }

func loadAutoconnect() (client.Config, bool) {
	var cfg client.Config
	b, err := os.ReadFile(clientAutoconnectFile)
	if err != nil {
		return cfg, false
	}
	if err := json.Unmarshal(b, &cfg); err != nil || cfg.SSID == "" {
		return cfg, false
	}
	return cfg, true
}

// systemModeSwapHandler flips the instance between AP (server) and client roles:
// it persists the target role, then (in the background) installs that role's
// dependencies and restarts the service so it comes back up in the new mode. The
// console polls status and reloads once the new mode is live.
func systemModeSwapHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Mode string `json:"mode"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		target := strings.ToLower(strings.TrimSpace(body.Mode))
		if target == "server" {
			target = "ap"
		}
		if target != "ap" && target != "client" {
			api.WriteErr(w, http.StatusBadRequest, "mode must be 'ap' or 'client'")
			return
		}
		api.WriteJSON(w, map[string]any{"status": "switching", "mode": target})
		go swapModeAsync(target)
	}
}

// swapModeAsync installs the target role's dependencies, persists the new role,
// then restarts the service into it. It runs in the background so the HTTP
// response returns first. The mode file is written only after deps are in place
// and right before the restart, so status keeps reporting the current role until
// the new one is actually live (and a failed dep install leaves the role unchanged).
func swapModeAsync(target string) {
	time.Sleep(500 * time.Millisecond)
	if target == "client" {
		_ = deps.InstallPackages(clientDepPackages)
	} else {
		_ = deps.VerifyAndInstall()
	}
	_ = os.WriteFile(modeOverrideFile, []byte(target), 0o644)
	_ = exec.Command("systemctl", "restart", installUnitName).Run()
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
		cfg := clientConfigFromNetwork(app, rec)

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
		client.Get().SetReconnect(false, 0, 0) // a new connection clears any prior cycle
		go func() { _ = client.Get().Connect(cfg) }()
		saveAutoconnect(cfg) // remember it so the client auto-reconnects after a reboot/crash
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
		client.Get().SetReconnect(false, 0, 0)
		client.Get().Stop()
		clearAutoconnect() // a deliberate disconnect should not auto-reconnect on next boot
		api.WriteJSON(w, client.Get().Status())
	}
}

// clientStatusHandler returns the live client status for the console.
func clientStatusHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		api.WriteJSON(w, client.Get().Status())
	}
}

// clientLogsHandler returns the client's full activity log for the live log window.
func clientLogsHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		api.WriteJSON(w, map[string]any{"lines": client.Get().Logs()})
	}
}

// clientReconnectHandler toggles reconnect cycling (handshake capture): the client
// periodically deauths and reassociates so students can capture WPA handshakes.
func clientReconnectHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Enabled          bool    `json:"enabled"`
			FrequencySeconds float64 `json:"frequency_seconds"`
			JitterSeconds    float64 `json:"jitter_seconds"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		freq := time.Duration(body.FrequencySeconds * float64(time.Second))
		jitter := time.Duration(body.JitterSeconds * float64(time.Second))
		client.Get().SetReconnect(body.Enabled, freq, jitter)
		api.WriteJSON(w, client.Get().Status())
	}
}
