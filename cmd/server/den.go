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
	"context"
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
	"os"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	"github.com/vtemlabs/tala-wte/internal/api"
	"github.com/vtemlabs/tala-wte/internal/client"
	"github.com/vtemlabs/tala-wte/internal/updater"
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
func memberHTTPClient(fp string, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
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
	return memberHTTPClient(fp, 15*time.Second).Do(req)
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

// denUpdateHandler updates the whole pack from the leader. Members may be a mix
// of amd64 and arm64, so the leader downloads each needed architecture's build
// once and pushes the matching, checksum-verified binary to each member over the
// agent channel; a member never needs its own internet access. A member that does
// not report its architecture (older build) or that lacks the push endpoint falls
// back to pulling the release from GitHub itself.
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

		// Download each architecture once and reuse it across members.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		type build struct{ path, ver, sha string }
		builds := map[string]*build{}
		defer func() {
			for _, b := range builds {
				_ = os.Remove(b.path)
			}
		}()
		buildFor := func(arch string) (*build, error) {
			if b, ok := builds[arch]; ok {
				return b, nil
			}
			p, v, s, e := updater.DownloadAsset(ctx, arch)
			if e != nil {
				return nil, e
			}
			b := &build{path: p, ver: v, sha: s}
			builds[arch] = b
			return b, nil
		}

		results := make([]updateResult, 0, len(members))
		for _, m := range members {
			res := updateResult{Name: m.GetString("name")}
			arch := memberArch(app, m)
			if arch == "" {
				// Architecture unknown (offline or older build): let the member pull.
				res.OK, res.Detail = pullUpdate(app, m)
				results = append(results, res)
				continue
			}
			b, e := buildFor(arch)
			if e != nil {
				res.Detail = "could not fetch " + arch + " build: " + e.Error()
				results = append(results, res)
				continue
			}
			if e := pushToMember(app, m, b.path, b.ver, b.sha, arch); e != nil {
				res.Detail = e.Error()
			} else {
				res.OK = true
				res.Detail = "pushed " + b.ver + " (" + arch + ")"
			}
			results = append(results, res)
		}
		api.WriteJSON(w, map[string]any{"results": results})
	}
}

// memberArch asks a member for its CPU architecture via its status (reachable
// with the agent key). It returns "" when the member is unreachable or runs an
// older build that does not report one.
func memberArch(app *pocketbase.PocketBase, member *core.Record) string {
	resp, err := memberCall(app, member, http.MethodGet, "/api/wte/client/status", nil)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return ""
	}
	var st struct {
		Arch string `json:"arch"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&st)
	return st.Arch
}

// pullUpdate tells a member to update itself from GitHub (the pre-push path).
func pullUpdate(app *pocketbase.PocketBase, member *core.Record) (bool, string) {
	resp, err := memberCall(app, member, http.MethodPost, "/api/wte/system/update", nil)
	if err != nil {
		return false, "unreachable: " + err.Error()
	}
	defer resp.Body.Close()
	var body struct {
		Version string `json:"version"`
		Error   string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if resp.StatusCode >= 300 {
		if body.Error != "" {
			return false, body.Error
		}
		return false, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return true, "updating to " + body.Version + " (pulled)"
}

// pushToMember streams a downloaded, checksum-verified binary to a member's
// /system/apply over the pinned agent channel. A member predating push support
// answers 404, in which case it falls back to a pull update.
func pushToMember(app *pocketbase.PocketBase, member *core.Record, binPath, ver, sha, arch string) error {
	f, err := os.Open(binPath)
	if err != nil {
		return err
	}
	defer f.Close()
	fp := member.GetString("cert_fingerprint")
	req, err := http.NewRequest(http.MethodPost, memberBaseURL(member.GetString("address"))+"/api/wte/system/apply", f)
	if err != nil {
		return err
	}
	if fi, e := f.Stat(); e == nil {
		req.ContentLength = fi.Size()
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Agent-Key", member.GetString("agent_key"))
	req.Header.Set("X-Update-SHA256", sha)
	req.Header.Set("X-Update-Version", ver)
	req.Header.Set("X-Update-Arch", arch)
	resp, err := memberHTTPClient(fp, 10*time.Minute).Do(req)
	if err != nil {
		return fmt.Errorf("unreachable: %w", err)
	}
	defer resp.Body.Close()
	// Trust-on-first-use: record the member's certificate on first contact.
	if fp == "" && resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		sum := sha256.Sum256(resp.TLS.PeerCertificates[0].Raw)
		member.Set("cert_fingerprint", hex.EncodeToString(sum[:]))
		_ = app.Save(member)
	}
	if resp.StatusCode == http.StatusNotFound {
		ok, detail := pullUpdate(app, member)
		if ok {
			return nil
		}
		return fmt.Errorf("push unsupported; pull fallback: %s", detail)
	}
	if resp.StatusCode >= 300 {
		var body struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&body)
		if body.Error != "" {
			return fmt.Errorf("member rejected push: %s", body.Error)
		}
		return fmt.Errorf("member rejected push (HTTP %d)", resp.StatusCode)
	}
	return nil
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
