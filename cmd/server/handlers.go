// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"

	tala "github.com/vtemlabs/tala-wte"
	"github.com/vtemlabs/tala-wte/internal/api"
	"github.com/vtemlabs/tala-wte/internal/capture"
	"github.com/vtemlabs/tala-wte/internal/deps"
	"github.com/vtemlabs/tala-wte/internal/iface"
	"github.com/vtemlabs/tala-wte/internal/ldap"
	"github.com/vtemlabs/tala-wte/internal/portal"
	"github.com/vtemlabs/tala-wte/internal/sim"
)

// staticHandler serves the embedded SvelteKit build.
func staticHandler() func(http.ResponseWriter, *http.Request) {
	subFS, err := fs.Sub(tala.FrontendFS, "web/build")
	if err != nil {
		panic("CRITICAL ERROR: Embedded SvelteKit UI bundle could not be initialized")
	}
	fileServer := http.FileServer(http.FS(subFS))

	return func(w http.ResponseWriter, r *http.Request) {
		reqPath := r.PathValue("path")
		if reqPath == "" {
			reqPath = "index.html"
		}

		clean := path.Clean("/" + reqPath)
		clean = strings.TrimPrefix(clean, "/")

		if _, err := fs.Stat(subFS, clean); err != nil {
			// Fall through to the SPA entry point for unknown paths.
			r.URL.Path = "/"
		}

		fileServer.ServeHTTP(w, r)
	}
}

// interfacesHandler returns available wireless interfaces; in_use maps claimed adapters to their network's SSID.
func interfacesHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Tala WTE does not support virtual/simulated adapters; expose real radios only.
		ifaces := make([]iface.Adapter, 0)
		for _, a := range iface.DiscoverAdapters() {
			if !iface.IsVirtualDriver(a.Driver) {
				ifaces = append(ifaces, a)
			}
		}
		api.WriteJSON(w, map[string]any{
			"interfaces": ifaces,
			"in_use":     sim.InUseInterfaces(),
		})
	}
}

// systemStatusHandler returns aggregate system status.
func systemStatusHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Setup is needed until a real admin (not the install placeholder) is created.
		realSuperuser, _ := hasRealSuperuser(app)
		needsSetup := !realSuperuser

		radiusRunning := false
		if err := exec.Command("pgrep", "-x", "freeradius").Run(); err == nil {
			radiusRunning = true
		}

		adapters := iface.DiscoverAdapters()
		realCount := 0
		for _, a := range adapters {
			if !iface.IsVirtualDriver(a.Driver) {
				realCount++
			}
		}

		mode := "ap"
		if clientMode() {
			mode = "client"
		}
		api.WriteJSON(w, map[string]any{
			"status":             "ok",
			"mode":               mode,
			"needs_setup":        needsSetup,
			"radius_running":     radiusRunning,
			"ldap_running":       ldap.IsRunning(),
			"interface_count":    len(adapters),
			"real_adapter_count": realCount,
		})
	}
}

// saveSetting persists a key/value pair to the settings collection.
func saveSetting(app *pocketbase.PocketBase, key, value string) error {
	record, err := app.FindFirstRecordByFilter("settings", "key = {:key}", map[string]any{"key": key})
	if err != nil {
		col, colErr := app.FindCollectionByNameOrId("settings")
		if colErr != nil {
			return fmt.Errorf("settings collection not found: %w", colErr)
		}
		record = core.NewRecord(col)
		record.Set("key", key)
	}
	record.Set("value", value)
	return app.Save(record)
}

// loadSetting reads a value from the settings collection.
func loadSetting(app *pocketbase.PocketBase, key string) string {
	record, err := app.FindFirstRecordByFilter("settings", "key = {:key}", map[string]any{"key": key})
	if err != nil {
		return ""
	}
	return record.GetString("value")
}

// hydrateSettingEnv copies a persisted setting into its environment variable at boot; an explicit env override always wins.
func hydrateSettingEnv(app *pocketbase.PocketBase, key, env string) {
	if os.Getenv(env) != "" {
		return
	}
	if v := loadSetting(app, key); v != "" {
		os.Setenv(env, v)
	}
}

// legalPageHandler serves the Terms / Acceptable Use / Privacy pages so portal-preview links resolve. Public by design.
func legalPageHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		html, ok := portal.LegalPageHTML(r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, html)
	}
}

// licenseHandler serves the embedded Tala WTE license as plain text. Public: it must be readable before an admin account exists.
func licenseHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, tala.LicenseText)
	}
}

// settingsSaveHandler persists system settings (uplink interface, regulatory domain).
func settingsSaveHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			UplinkIface string `json:"uplink_iface"`
			CountryCode string `json:"country_code"`
			APSubnet    string `json:"ap_subnet"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			api.WriteErr(w, http.StatusBadRequest, "invalid json")
			return
		}

		if req.UplinkIface != "" {
			os.Setenv("TALA_UPLINK_IFACE", req.UplinkIface)
			if err := saveSetting(app, "uplink_iface", req.UplinkIface); err != nil {
				log.Printf("[settings] failed to persist uplink_iface: %v", err)
				api.WriteErr(w, http.StatusInternalServerError, "failed to save uplink interface setting")
				return
			}
		}

		// The regulatory domain decides which channels/bands hostapd may beacon on, so apply it to the live radio immediately.
		cc := strings.ToUpper(strings.TrimSpace(req.CountryCode))
		if cc != "" {
			if !regexp.MustCompile(`^[A-Z]{2}$`).MatchString(cc) {
				api.WriteErr(w, http.StatusBadRequest, "country code must be two letters (e.g. US, GB, DE)")
				return
			}
			os.Setenv("TALA_COUNTRY_CODE", cc)
			if err := saveSetting(app, "country_code", cc); err != nil {
				log.Printf("[settings] failed to persist country_code: %v", err)
				api.WriteErr(w, http.StatusInternalServerError, "failed to save regulatory domain setting")
				return
			}
			if err := deps.ApplyRegDomain(cc); err != nil {
				// Not fatal: the choice is persisted and applies on next boot.
				log.Printf("[settings] iw reg set %s failed (persisted for next boot): %v", cc, err)
			}
		}

		// Default AP/LAN subnet for new networks; each network can override it on creation.
		subnet := strings.TrimSpace(req.APSubnet)
		if subnet != "" {
			if _, _, err := net.ParseCIDR(subnet); err != nil {
				api.WriteErr(w, http.StatusBadRequest, "subnet must be CIDR notation (e.g. 10.0.0.0/24)")
				return
			}
			if err := saveSetting(app, "ap_subnet", subnet); err != nil {
				log.Printf("[settings] failed to persist ap_subnet: %v", err)
				api.WriteErr(w, http.StatusInternalServerError, "failed to save subnet setting")
				return
			}
		}

		api.WriteJSON(w, map[string]any{
			"status":       "saved",
			"uplink_iface": req.UplinkIface,
			"country_code": cc,
			"ap_subnet":    subnet,
		})
	}
}

// settingsGetHandler returns current system settings.
func settingsGetHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		uplinkIface := os.Getenv("TALA_UPLINK_IFACE")
		if uplinkIface == "" {
			uplinkIface = loadSetting(app, "uplink_iface")
			if uplinkIface != "" {
				os.Setenv("TALA_UPLINK_IFACE", uplinkIface)
			}
		}
		countryCode := os.Getenv("TALA_COUNTRY_CODE")
		if countryCode == "" {
			countryCode = loadSetting(app, "country_code")
			if countryCode == "" {
				countryCode = "US" // matches the backend default in regulatoryCountry()
			}
			os.Setenv("TALA_COUNTRY_CODE", countryCode)
		}
		apSubnet := loadSetting(app, "ap_subnet")
		if apSubnet == "" {
			apSubnet = "10.0.0.0/24" // historical default
		}
		api.WriteJSON(w, map[string]any{
			"uplink_iface": uplinkIface,
			"country_code": countryCode,
			"ap_subnet":    apSubnet,
		})
	}
}

// radiusConfigHandler saves RADIUS configuration.
func radiusConfigHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			EAPType      string `json:"eap_type"`
			InnerAuth    string `json:"inner_auth"`
			SharedSecret string `json:"shared_secret"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			api.WriteErr(w, http.StatusBadRequest, "invalid json")
			return
		}

		if req.SharedSecret != "" {
			safeSecret := regexp.MustCompile(`^[a-zA-Z0-9!@#$%^&*()_+\-=\[\]{}:,.<>?/~]+$`)
			if !safeSecret.MatchString(req.SharedSecret) || strings.ContainsAny(req.SharedSecret, "\n\r") {
				api.WriteErr(w, http.StatusBadRequest, "shared secret contains invalid characters")
				return
			}
		}

		if req.SharedSecret != "" {
			os.Setenv("TALA_RADIUS_SECRET", req.SharedSecret)
			if err := saveSetting(app, "radius_secret", req.SharedSecret); err != nil {
				log.Printf("[radius] failed to persist shared secret: %v", err)
				api.WriteErr(w, http.StatusInternalServerError, "failed to save RADIUS shared secret")
				return
			}
		}
		if req.EAPType != "" {
			if err := saveSetting(app, "radius_eap_type", req.EAPType); err != nil {
				log.Printf("[radius] failed to persist eap_type: %v", err)
				api.WriteErr(w, http.StatusInternalServerError, "failed to save EAP type")
				return
			}
		}
		if req.InnerAuth != "" {
			if err := saveSetting(app, "radius_inner_auth", req.InnerAuth); err != nil {
				log.Printf("[radius] failed to persist inner_auth: %v", err)
				api.WriteErr(w, http.StatusInternalServerError, "failed to save inner auth method")
				return
			}
		}

		if req.SharedSecret != "" {
			clientsConf := fmt.Sprintf("client localhost {\n\tipaddr = 127.0.0.1\n\tsecret = %s\n}\n", req.SharedSecret)
			if err := os.WriteFile("/etc/freeradius/3.0/clients.conf", []byte(clientsConf), 0o640); err != nil {
				log.Printf("[radius] failed to write clients.conf: %v", err)
				api.WriteErr(w, http.StatusInternalServerError, "failed to write FreeRADIUS config")
				return
			}
		}

		if err := exec.Command("systemctl", "restart", "freeradius").Run(); err != nil {
			log.Printf("[radius] failed to restart freeradius: %v", err)
			api.WriteErr(w, http.StatusInternalServerError, "FreeRADIUS failed to restart")
			return
		}

		log.Printf("[radius] configuration saved and freeradius restarted (eap=%s inner=%s)", req.EAPType, req.InnerAuth)
		api.WriteJSON(w, map[string]any{
			"status":     "saved",
			"eap_type":   req.EAPType,
			"inner_auth": req.InnerAuth,
		})
	}
}

// captureStartHandler starts a packet capture; the PocketBase record id doubles as the capture session id.
func captureStartHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			NetworkID string `json:"network_id"`
			Layer     string `json:"layer"`
			Interface string `json:"interface"`
			Filter    string `json:"filter"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			api.WriteErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		if req.NetworkID == "" || req.Interface == "" {
			api.WriteErr(w, http.StatusBadRequest, "network_id and interface required")
			return
		}

		if err := capture.ValidateBPFFilter(req.Filter); err != nil {
			api.WriteErr(w, http.StatusBadRequest, err.Error())
			return
		}

		layer := capture.LayerNetwork
		if req.Layer == "wireless" {
			layer = capture.LayerWireless
		}

		col, err := app.FindCollectionByNameOrId("captures")
		if err != nil {
			api.WriteErr(w, http.StatusInternalServerError, "captures collection missing")
			return
		}
		record := core.NewRecord(col)
		record.Set("network_id", req.NetworkID)
		record.Set("layer", string(layer))
		record.Set("interface", req.Interface)
		record.Set("filter", req.Filter)
		record.Set("status", "running")
		record.Set("started_at", types.NowDateTime())
		if err := app.Save(record); err != nil {
			api.WriteErr(w, http.StatusInternalServerError, "create capture record: "+err.Error())
			return
		}

		captureID := record.Id

		onExit := func(packetCount int) {
			rec, err := app.FindRecordById("captures", captureID)
			if err != nil {
				log.Printf("[capture] post-exit lookup failed for %s: %v", captureID, err)
				return
			}
			rec.Set("status", "stopped")
			rec.Set("stopped_at", types.NowDateTime())
			rec.Set("packet_count", packetCount)
			// Do not set "file": it is a File-type field; a string filename fails validation. The pcap is located by record id.
			if err := app.Save(rec); err != nil {
				log.Printf("[capture] post-exit save failed for %s: %v", captureID, err)
			}
		}

		// A running network moves its AP interface into the "wte-<networkID>" namespace; capture inside it. Falls back to the host if absent.
		netns := "wte-" + req.NetworkID
		sess, err := capture.Start(captureID, req.Interface, netns, layer, req.Filter, onExit)
		if err != nil {
			record.Set("status", "error")
			_ = app.Save(record)
			api.WriteErr(w, http.StatusInternalServerError, err.Error())
			return
		}

		api.WriteJSON(w, map[string]any{
			"status": "started",
			"id":     captureID,
			"file":   sess.FilePath,
		})
	}
}

// captureStopHandler stops a packet capture and updates the record with stopped_at and the final packet count.
func captureStopHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		// Wait for the pcap to finalize; the background exit goroutine is unreliable under PocketBase, so do not depend on it.
		count, err := capture.StopAndWait(id, 6*time.Second)
		if err != nil {
			api.WriteErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		// Do not set the "file" field: a string filename fails File-field validation. The pcap is located by record id.
		if rec, ferr := app.FindRecordById("captures", id); ferr == nil {
			rec.Set("status", "stopped")
			rec.Set("stopped_at", types.NowDateTime())
			rec.Set("packet_count", count)
			if serr := app.Save(rec); serr != nil {
				log.Printf("[capture] stop record update failed for %s: %v", id, serr)
			}
		}
		api.WriteJSON(w, map[string]any{"status": "stopped", "id": id, "packet_count": count})
	}
}

// captureDownloadHandler streams the pcap file for a completed capture.
func captureDownloadHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !safeCaptureID.MatchString(id) {
			api.WriteErr(w, http.StatusBadRequest, "invalid capture id")
			return
		}
		// Confirm the record exists before serving, to prevent enumeration of files on disk.
		if _, err := app.FindRecordById("captures", id); err != nil {
			api.WriteErr(w, http.StatusNotFound, "capture not found")
			return
		}
		pcapPath := filepath.Join(capture.CaptureDir, id+".pcapng")
		if _, err := os.Stat(pcapPath); err != nil {
			api.WriteErr(w, http.StatusNotFound, "capture file missing")
			return
		}
		w.Header().Set("Content-Type", "application/vnd.tcpdump.pcap")
		w.Header().Set("Content-Disposition", `attachment; filename="`+id+`.pcapng"`)
		http.ServeFile(w, r, pcapPath)
	}
}

// captureAnalyzeHandler returns a structured analysis of a completed capture (protocol mix, top talkers, DNS, HTTP, cleartext creds).
func captureAnalyzeHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !safeCaptureID.MatchString(id) {
			api.WriteErr(w, http.StatusBadRequest, "invalid capture id")
			return
		}
		if _, err := app.FindRecordById("captures", id); err != nil {
			api.WriteErr(w, http.StatusNotFound, "capture not found")
			return
		}
		result, err := capture.Analyze(id)
		if err != nil {
			api.WriteErr(w, http.StatusNotFound, "capture file missing or unreadable")
			return
		}
		api.WriteJSON(w, result)
	}
}

// capturePacketsHandler returns the packet list for a capture, optionally
// narrowed by a Wireshark display filter (the ?filter= query parameter).
func capturePacketsHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !safeCaptureID.MatchString(id) {
			api.WriteErr(w, http.StatusBadRequest, "invalid capture id")
			return
		}
		if _, err := app.FindRecordById("captures", id); err != nil {
			api.WriteErr(w, http.StatusNotFound, "capture not found")
			return
		}
		filter := r.URL.Query().Get("filter")
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		rows, truncated, err := capture.Packets(id, filter, limit)
		if err != nil {
			api.WriteErr(w, http.StatusBadRequest, err.Error())
			return
		}
		api.WriteJSON(w, map[string]any{"packets": rows, "truncated": truncated, "count": len(rows)})
	}
}

// capturePacketDetailHandler returns the full verbose dissection of one frame.
func capturePacketDetailHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !safeCaptureID.MatchString(id) {
			api.WriteErr(w, http.StatusBadRequest, "invalid capture id")
			return
		}
		if _, err := app.FindRecordById("captures", id); err != nil {
			api.WriteErr(w, http.StatusNotFound, "capture not found")
			return
		}
		n, _ := strconv.Atoi(r.PathValue("n"))
		detail, err := capture.PacketDetail(id, n)
		if err != nil {
			api.WriteErr(w, http.StatusBadRequest, "could not read packet")
			return
		}
		api.WriteJSON(w, map[string]any{"detail": detail})
	}
}

var safeCaptureID = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

// portalBundleDir is where multi-file (zip) portal bundles are extracted, served via the "fs:<slug>" reference.
const portalBundleDir = "/var/lib/tala-wte/portals"

var slugSanitize = regexp.MustCompile(`[^a-z0-9]+`)

// portalTemplatesHandler returns the embedded built-in template gallery.
func portalTemplatesHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		templates, err := portal.Catalog()
		if err != nil {
			api.WriteErr(w, http.StatusInternalServerError, "failed to load templates")
			return
		}
		api.WriteJSON(w, map[string]any{"templates": templates})
	}
}

// portalUploadHandler accepts an uploaded portal template: a single .html file (inline) or a .zip bundle (extracted to portalBundleDir).
func portalUploadHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(25 << 20); err != nil { // 25 MB cap
			api.WriteErr(w, http.StatusBadRequest, "invalid upload (max 25 MB)")
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			api.WriteErr(w, http.StatusBadRequest, "missing file")
			return
		}
		defer file.Close()

		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			name = strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
		}
		if name == "" {
			name = "Uploaded Portal"
		}

		col, err := app.FindCollectionByNameOrId("portals")
		if err != nil {
			api.WriteErr(w, http.StatusInternalServerError, "portals collection missing")
			return
		}
		rec := core.NewRecord(col)
		rec.Set("name", name)
		rec.Set("type", "custom")
		rec.Set("category", "custom")

		lower := strings.ToLower(header.Filename)
		switch {
		case strings.HasSuffix(lower, ".html"), strings.HasSuffix(lower, ".htm"):
			data, err := io.ReadAll(io.LimitReader(file, 25<<20))
			if err != nil {
				api.WriteErr(w, http.StatusInternalServerError, "failed to read file")
				return
			}
			if !strings.Contains(strings.ToLower(string(data)), "<html") && !strings.Contains(strings.ToLower(string(data)), "<!doctype") {
				api.WriteErr(w, http.StatusBadRequest, "file does not look like HTML")
				return
			}
			rec.Set("html", string(data))

		case strings.HasSuffix(lower, ".zip"):
			slug, err := uniquePortalSlug(app, name)
			if err != nil {
				api.WriteErr(w, http.StatusInternalServerError, err.Error())
				return
			}
			destDir := filepath.Join(portalBundleDir, slug)
			if err := extractPortalZip(file, header.Size, destDir); err != nil {
				_ = os.RemoveAll(destDir)
				api.WriteErr(w, http.StatusBadRequest, err.Error())
				return
			}
			rec.Set("slug", slug)
			rec.Set("html", "fs:"+slug)

		default:
			api.WriteErr(w, http.StatusBadRequest, "unsupported file type (use .html or .zip)")
			return
		}

		if err := app.Save(rec); err != nil {
			api.WriteErr(w, http.StatusInternalServerError, "failed to save portal: "+err.Error())
			return
		}
		api.WriteJSON(w, map[string]any{"id": rec.Id, "name": name})
	}
}

// portalScrapeHandler clones a live page by URL (SSRF guarded), inlines its assets, and saves it as a custom portal.
func portalScrapeHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			URL  string `json:"url"`
			Name string `json:"name"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&body); err != nil {
			api.WriteErr(w, http.StatusBadRequest, "invalid request body")
			return
		}
		body.URL = strings.TrimSpace(body.URL)
		if body.URL == "" {
			api.WriteErr(w, http.StatusBadRequest, "url is required")
			return
		}

		htmlStr, err := portal.Scrape(body.URL)
		if err != nil {
			api.WriteErr(w, http.StatusBadGateway, "scrape failed: "+err.Error())
			return
		}
		if len(htmlStr) > portalHTMLMax {
			api.WriteErr(w, http.StatusRequestEntityTooLarge,
				fmt.Sprintf("scraped page is too large (%d bytes); try a simpler page", len(htmlStr)))
			return
		}

		name := strings.TrimSpace(body.Name)
		if name == "" {
			if u, e := url.Parse(body.URL); e == nil {
				name = u.Host
			}
			if name == "" {
				name = "Scraped Portal"
			}
		}

		col, err := app.FindCollectionByNameOrId("portals")
		if err != nil {
			api.WriteErr(w, http.StatusInternalServerError, "portals collection missing")
			return
		}
		rec := core.NewRecord(col)
		rec.Set("name", name)
		rec.Set("type", "custom")
		rec.Set("category", "custom")
		rec.Set("description", "Scraped from "+body.URL)
		rec.Set("html", htmlStr)
		if err := app.Save(rec); err != nil {
			api.WriteErr(w, http.StatusInternalServerError, "failed to save portal: "+err.Error())
			return
		}
		api.WriteJSON(w, map[string]any{"id": rec.Id, "name": name})
	}
}

// portalPreviewHandler serves a portal for in-UI iframe preview. Unauthenticated (iframes cannot attach the auth header) but only serves non-sensitive portal content with strict path containment.
func portalPreviewHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		rec, err := app.FindRecordById("portals", id)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		// Scope this route to same-origin framing (api.WriteJSON sets DENY globally).
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")

		html := rec.GetString("html")
		if !strings.HasPrefix(html, "fs:") {
			// Normalize so the preview matches what a connecting client gets live.
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(portal.Normalize(html)))
			return
		}

		baseDir := filepath.Join(portalBundleDir, strings.TrimPrefix(html, "fs:"))
		reqPath := r.PathValue("path")
		if reqPath == "" {
			reqPath = "index.html"
		}
		target := filepath.Join(baseDir, filepath.Clean("/"+reqPath))
		if target != baseDir && !strings.HasPrefix(target, filepath.Clean(baseDir)+string(os.PathSeparator)) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if stat, err := os.Stat(target); err == nil && !stat.IsDir() {
			http.ServeFile(w, r, target)
			return
		}
		http.ServeFile(w, r, filepath.Join(baseDir, "index.html"))
	}
}

// uniquePortalSlug derives a filesystem-safe, unique slug from a portal name.
func uniquePortalSlug(app *pocketbase.PocketBase, name string) (string, error) {
	base := strings.Trim(slugSanitize.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if base == "" {
		base = "portal"
	}
	slug := base
	for i := 1; i < 1000; i++ {
		if _, err := os.Stat(filepath.Join(portalBundleDir, slug)); os.IsNotExist(err) {
			existing, _ := app.FindFirstRecordByFilter("portals", "slug = {:slug}", map[string]any{"slug": slug})
			if existing == nil {
				return slug, nil
			}
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
	return "", fmt.Errorf("could not allocate a unique slug for %q", name)
}

// extractPortalZip safely extracts a zip into destDir (zip-slip protected, common top-level dir stripped), requiring an index.html at the root.
func extractPortalZip(file io.ReaderAt, size int64, destDir string) error {
	zr, err := zip.NewReader(file, size)
	if err != nil {
		return fmt.Errorf("invalid zip archive")
	}

	prefix := commonZipPrefix(zr.File)

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("failed to create bundle directory")
	}
	cleanBase := filepath.Clean(destDir)
	wrote := 0
	for _, f := range zr.File {
		rel := strings.TrimPrefix(f.Name, prefix)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" || strings.HasPrefix(filepath.Base(rel), ".") {
			continue
		}
		target := filepath.Join(destDir, filepath.Clean("/"+rel))
		if target != cleanBase && !strings.HasPrefix(target, cleanBase+string(os.PathSeparator)) {
			return fmt.Errorf("archive contains an unsafe path")
		}
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(target, 0o755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("failed to create directory")
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("failed to read archive entry")
		}
		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return fmt.Errorf("failed to write file")
		}
		if _, err := io.Copy(out, io.LimitReader(rc, 25<<20)); err != nil {
			out.Close()
			rc.Close()
			return fmt.Errorf("failed to extract file")
		}
		out.Close()
		rc.Close()
		wrote++
	}
	if wrote == 0 {
		return fmt.Errorf("archive is empty")
	}
	indexPath := filepath.Join(destDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		return fmt.Errorf("archive must contain an index.html at its root")
	}
	// Normalize the bundle's index.html once here rather than on every request, since fs: bundles are served as static files.
	if b, err := os.ReadFile(indexPath); err == nil {
		if normalized := portal.Normalize(string(b)); normalized != string(b) {
			_ = os.WriteFile(indexPath, []byte(normalized), 0o644)
		}
	}
	return nil
}

// commonZipPrefix returns a single shared top-level directory prefix (trailing slash) if every entry lives under it, otherwise "".
func commonZipPrefix(files []*zip.File) string {
	prefix := ""
	for _, f := range files {
		name := strings.TrimPrefix(f.Name, "/")
		if name == "" {
			continue
		}
		idx := strings.Index(name, "/")
		if idx < 0 {
			return "" // a file at the root means no single common dir
		}
		top := name[:idx+1]
		if prefix == "" {
			prefix = top
		} else if prefix != top {
			return ""
		}
	}
	return prefix
}
