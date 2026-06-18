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
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("[den] agent key generation failed (no system entropy): %v", err)
	}
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
		stored := storedAgentKey(app)
		if key != "" && stored != "" && subtle.ConstantTimeCompare([]byte(key), []byte(stored)) == 1 {
			h(e.Response, e.Request)
			return nil
		}
		api.WriteErr(e.Response, http.StatusForbidden, "agent key or superuser auth required")
		return nil
	}
}

// ---- leader side: reach members ----

// memberHTTPClient talks to a den member over its self-signed HTTPS, pinning the
// member's leaf certificate to the expected SHA-256 fingerprint instead of a CA
// chain. An empty fp means trust-on-first-use: the connection is allowed and the
// caller records the fingerprint it observed, so a later MITM that swaps the
// certificate is rejected.
func memberHTTPClient(fp string) *http.Client {
	return &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			// Self-signed members have no CA chain, so default verification is
			// off and we pin the leaf fingerprint in VerifyConnection instead.
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				VerifyConnection: func(cs tls.ConnectionState) error {
					if len(cs.PeerCertificates) == 0 {
						return fmt.Errorf("member presented no certificate")
					}
					if fp == "" {
						return nil // trust on first use; caller persists the observed fingerprint
					}
					sum := sha256.Sum256(cs.PeerCertificates[0].Raw)
					if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(fp)) != 1 {
						return fmt.Errorf("member certificate fingerprint mismatch (possible MITM); remove and re-add the member to re-pin")
					}
					return nil
				},
			},
		},
	}
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

func memberRequest(method, base, path, key, fp string, body any) (*http.Response, error) {
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
	return memberHTTPClient(fp).Do(req)
}

// memberCall sends a request to a den member over its pinned channel using the
// member's stored agent key and certificate fingerprint. On first contact (no
// stored fingerprint) it pins trust-on-first-use and persists the fingerprint it
// observed, so later calls reject a swapped certificate.
func memberCall(app *pocketbase.PocketBase, member *core.Record, method, path string, body any) (*http.Response, error) {
	fp := member.GetString("cert_fingerprint")
	resp, err := memberRequest(method, memberBaseURL(member.GetString("address")), path, member.GetString("agent_key"), fp, body)
	if err != nil {
		return nil, err
	}
	if fp == "" && resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		sum := sha256.Sum256(resp.TLS.PeerCertificates[0].Raw)
		member.Set("cert_fingerprint", hex.EncodeToString(sum[:]))
		_ = app.Save(member)
	}
	return resp, nil
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
		resp, err := memberCall(app, member, http.MethodPost, "/api/wte/client/connect", clientConfigFromNetwork(net))
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
		go startMemberTrafficWhenConnected(base, key, member.GetString("cert_fingerprint"), opts, body.Reconnect)
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
func startMemberTrafficWhenConnected(base, key, fp string, opts client.TrafficOptions, rc *reconnectSettings) {
	for i := 0; i < 20; i++ {
		time.Sleep(3 * time.Second)
		sr, err := memberRequest(http.MethodGet, base, "/api/wte/client/status", key, fp, nil)
		if err != nil {
			continue
		}
		var st struct {
			Connected bool `json:"connected"`
		}
		_ = json.NewDecoder(sr.Body).Decode(&st)
		sr.Body.Close()
		if st.Connected {
			if r2, e := memberRequest(http.MethodPost, base, "/api/wte/client/start", key, fp, opts); e == nil {
				r2.Body.Close()
			}
			if rc != nil && rc.Enabled {
				if r3, e := memberRequest(http.MethodPost, base, "/api/wte/client/reconnect", key, fp, rc); e == nil {
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
		if resp, e := memberCall(app, member, http.MethodPost, "/api/wte/client/disconnect", nil); e == nil {
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
		resp, err := memberCall(app, member, http.MethodGet, "/api/wte/client/status", nil)
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

// denUpdateHandler tells every reachable member to pull and apply the latest
// release so the leader and its pack stay on matching versions. Each member runs
// its own in-app update and restarts; results report per member.
func denUpdateHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		members, err := app.FindAllRecords("den_members")
		if err != nil {
			api.WriteErr(w, http.StatusInternalServerError, "could not list den members")
			return
		}
		type updateResult struct {
			Name   string `json:"name"`
			OK     bool   `json:"ok"`
			Detail string `json:"detail"`
		}
		results := make([]updateResult, 0, len(members))
		for _, m := range members {
			res := updateResult{Name: m.GetString("name")}
			resp, e := memberCall(app, m, http.MethodPost, "/api/wte/system/update", nil)
			if e != nil {
				res.Detail = "unreachable: " + e.Error()
			} else {
				var body struct {
					Version string `json:"version"`
					Error   string `json:"error"`
				}
				_ = json.NewDecoder(resp.Body).Decode(&body)
				resp.Body.Close()
				if resp.StatusCode >= 300 {
					res.Detail = body.Error
					if res.Detail == "" {
						res.Detail = fmt.Sprintf("HTTP %d", resp.StatusCode)
					}
				} else {
					res.OK = true
					res.Detail = "updating to " + body.Version
				}
			}
			results = append(results, res)
		}
		api.WriteJSON(w, map[string]any{"results": results})
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
		rec := m
		go func() {
			if resp, e := memberCall(app, rec, http.MethodPost, "/api/wte/client/disconnect", nil); e == nil {
				resp.Body.Close()
			}
			rec.Set("network_id", "")
			_ = app.Save(rec)
		}()
	}
}
