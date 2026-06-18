// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government use
// require a license from VTEM Labs. See the LICENSE file.

package main

// The "den": a server (AP) acts as the den leader and drives a pack of client
// instances (members). Each member exposes an agent key; the leader registers the
// member by address + key, pushes a network's config to it, starts traffic, and
// stops it when the network goes away. The leader reaches members over their
// self-signed HTTPS using the agent key, so no member login is needed.

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/vtemlabs/tala-wte/internal/api"
	"github.com/vtemlabs/tala-wte/internal/client"
)

// ---- member side: agent key + auth ----

// newAgentKey returns a random control token a den leader uses to drive a member.
func newAgentKey() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func storedAgentKey(app *pocketbase.PocketBase) string {
	return loadSetting(app, "den_agent_key")
}

// ensureAgentKey returns this member's agent key, generating one on first use so
// it is always available to copy into a den leader.
func ensureAgentKey(app *pocketbase.PocketBase) string {
	if k := storedAgentKey(app); k != "" {
		return k
	}
	k := newAgentKey()
	_ = saveSetting(app, "den_agent_key", k)
	return k
}

// clientAgentKeyHandler returns this member's agent key (creating one if needed).
func clientAgentKeyHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		api.WriteJSON(w, map[string]any{"key": ensureAgentKey(app)})
	}
}

// clientAgentKeyRegenHandler rotates the agent key, invalidating any leader that
// still holds the old one.
func clientAgentKeyRegenHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		k := newAgentKey()
		_ = saveSetting(app, "den_agent_key", k)
		api.WriteJSON(w, map[string]any{"key": k})
	}
}

// wrapAgent admits a request from either a local superuser (the member's own
// console) or a den leader presenting the matching X-Agent-Key header.
func wrapAgent(app *pocketbase.PocketBase, h func(http.ResponseWriter, *http.Request)) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if e.Auth != nil && e.Auth.IsSuperuser() {
			h(e.Response, e.Request)
			return nil
		}
		key := e.Request.Header.Get("X-Agent-Key")
		if key != "" && key == storedAgentKey(app) {
			h(e.Response, e.Request)
			return nil
		}
		api.WriteErr(e.Response, http.StatusForbidden, "agent key or superuser auth required")
		return nil
	}
}

// ---- leader side: reach members ----

// denHTTPClient talks to member clients over their self-signed HTTPS.
var denHTTPClient = &http.Client{
	Timeout:   15 * time.Second,
	Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
}

// memberBaseURL normalizes a stored address into an https base URL, defaulting the
// scheme to https and the port to 8443.
func memberBaseURL(addr string) string {
	addr = strings.TrimSuffix(strings.TrimSpace(addr), "/")
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return addr
	}
	if !strings.Contains(addr, ":") {
		addr += ":8443"
	}
	return "https://" + addr
}

func memberRequest(method, base, path, key string, body any) (*http.Response, error) {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, base+path, rdr)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Agent-Key", key)
	return denHTTPClient.Do(req)
}

// clientConfigFromNetwork builds a client connection profile from a network record.
func clientConfigFromNetwork(rec *core.Record) client.Config {
	cfg := client.Config{
		SSID:       rec.GetString("ssid"),
		Protocol:   rec.GetString("protocol"),
		Passphrase: rec.GetString("passphrase"),
		Band:       rec.GetString("band"),
		Channel:    rec.GetInt("channel"),
		Hidden:     rec.GetBool("hidden"),
	}
	cfg.Portal.Enabled = rec.GetBool("portal_enabled")
	return cfg
}

// denDeployHandler assigns a member to a network and brings it online: it pushes
// the network's client config, waits for the member to associate, then starts
// traffic. The wait + start run in the background so the call returns promptly.
func denDeployHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		member, err := app.FindRecordById("den_members", r.PathValue("id"))
		if err != nil {
			api.WriteErr(w, http.StatusNotFound, "den member not found")
			return
		}
		var body struct {
			NetworkID string                 `json:"network_id"`
			Traffic   *client.TrafficOptions `json:"traffic"`
			Reconnect *reconnectSettings     `json:"reconnect"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		net, err := app.FindRecordById("networks", body.NetworkID)
		if err != nil {
			api.WriteErr(w, http.StatusBadRequest, "network not found")
			return
		}
		base := memberBaseURL(member.GetString("address"))
		key := member.GetString("agent_key")
		resp, err := memberRequest(http.MethodPost, base, "/api/wte/client/connect", key, clientConfigFromNetwork(net))
		if err != nil {
			api.WriteErr(w, http.StatusBadGateway, "could not reach member: "+err.Error())
			return
		}
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			api.WriteErr(w, http.StatusBadGateway, fmt.Sprintf("member rejected connect (HTTP %d)", resp.StatusCode))
			return
		}
		// The leader can push the full traffic config (every generator + target list
		// + credentials) and a reconnect-cycling schedule, exactly like configuring a
		// client by hand; default to a sensible standard mix when none is supplied.
		opts := client.TrafficOptions{Web: true, DNS: true, Ping: true, Local: true, Internet: true}
		if body.Traffic != nil {
			opts = *body.Traffic
		}
		member.Set("network_id", body.NetworkID)
		_ = app.Save(member)
		go startMemberTrafficWhenConnected(base, key, opts, body.Reconnect)
		api.WriteJSON(w, map[string]any{"status": "deploying"})
	}
}

// reconnectSettings is the handshake-capture cycling config a leader can push.
type reconnectSettings struct {
	Enabled          bool    `json:"enabled"`
	FrequencySeconds float64 `json:"frequency_seconds"`
	JitterSeconds    float64 `json:"jitter_seconds"`
}

// startMemberTrafficWhenConnected polls the member until it associates, then starts
// the requested traffic mix and, if asked, reconnect cycling.
func startMemberTrafficWhenConnected(base, key string, opts client.TrafficOptions, rc *reconnectSettings) {
	for i := 0; i < 20; i++ {
		time.Sleep(3 * time.Second)
		sr, err := memberRequest(http.MethodGet, base, "/api/wte/client/status", key, nil)
		if err != nil {
			continue
		}
		var st struct {
			Connected bool `json:"connected"`
		}
		_ = json.NewDecoder(sr.Body).Decode(&st)
		sr.Body.Close()
		if st.Connected {
			if r2, e := memberRequest(http.MethodPost, base, "/api/wte/client/start", key, opts); e == nil {
				r2.Body.Close()
			}
			if rc != nil && rc.Enabled {
				if r3, e := memberRequest(http.MethodPost, base, "/api/wte/client/reconnect", key, rc); e == nil {
					r3.Body.Close()
				}
			}
			return
		}
	}
}

// denStopHandler stops traffic, disconnects the member, and clears its assignment.
func denStopHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		member, err := app.FindRecordById("den_members", r.PathValue("id"))
		if err != nil {
			api.WriteErr(w, http.StatusNotFound, "den member not found")
			return
		}
		base := memberBaseURL(member.GetString("address"))
		key := member.GetString("agent_key")
		if resp, e := memberRequest(http.MethodPost, base, "/api/wte/client/disconnect", key, nil); e == nil {
			resp.Body.Close()
		}
		member.Set("network_id", "")
		_ = app.Save(member)
		api.WriteJSON(w, map[string]any{"status": "stopped"})
	}
}

// denStatusHandler proxies a member's live status so the den page can show it
// without the browser needing to reach each member directly.
func denStatusHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		member, err := app.FindRecordById("den_members", r.PathValue("id"))
		if err != nil {
			api.WriteErr(w, http.StatusNotFound, "den member not found")
			return
		}
		base := memberBaseURL(member.GetString("address"))
		key := member.GetString("agent_key")
		resp, err := memberRequest(http.MethodGet, base, "/api/wte/client/status", key, nil)
		if err != nil {
			api.WriteJSON(w, map[string]any{"reachable": false, "error": err.Error()})
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			api.WriteJSON(w, map[string]any{"reachable": false, "error": fmt.Sprintf("HTTP %d (agent key rejected?)", resp.StatusCode)})
			return
		}
		var st map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&st)
		api.WriteJSON(w, map[string]any{"reachable": true, "status": st})
	}
}

// teardownDenForNetwork disconnects every member assigned to a network. The leader
// calls this when the network stops or is deleted, so members stop chasing a
// network that no longer exists.
func teardownDenForNetwork(app *pocketbase.PocketBase, networkID string) {
	if networkID == "" {
		return
	}
	members, err := app.FindRecordsByFilter("den_members", "network_id = {:n}", "", 0, 0, map[string]any{"n": networkID})
	if err != nil {
		return
	}
	for _, m := range members {
		base := memberBaseURL(m.GetString("address"))
		key := m.GetString("agent_key")
		rec := m
		go func() {
			if resp, e := memberRequest(http.MethodPost, base, "/api/wte/client/disconnect", key, nil); e == nil {
				resp.Body.Close()
			}
			rec.Set("network_id", "")
			_ = app.Save(rec)
		}()
	}
}
