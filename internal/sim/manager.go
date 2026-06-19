// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

// Package sim manages the lifecycle of wireless networks via hostapd.
package sim

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	"github.com/vtemlabs/tala-wte/internal/api"
	"github.com/vtemlabs/tala-wte/internal/iface"
	"github.com/vtemlabs/tala-wte/internal/ldap"
	"github.com/vtemlabs/tala-wte/internal/netns"
	"github.com/vtemlabs/tala-wte/internal/portal"
	"github.com/vtemlabs/tala-wte/internal/routing"
	"github.com/vtemlabs/tala-wte/pkg/hostapd"
)

// PreflightHandler returns the enterprise readiness checklist (GET /api/wte/enterprise/preflight).
func PreflightHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		api.WriteJSON(w, CheckEnterprisePreflight())
	}
}

// ProvisionHandler runs AutoProvisionEnterprise and returns the per-step report (POST /api/wte/enterprise/provision).
func ProvisionHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		res := AutoProvisionEnterprise()
		api.WriteJSON(w, res)
	}
}

// resolveUplinkIface determines the uplink interface: TALA_UPLINK_IFACE, then default route, then eth0.
func resolveUplinkIface() string {
	if env := os.Getenv("TALA_UPLINK_IFACE"); env != "" {
		return env
	}
	out, err := exec.Command("ip", "route", "show", "default").Output()
	if err == nil {
		fields := strings.Fields(strings.TrimSpace(string(out)))
		for i, f := range fields {
			if f == "dev" && i+1 < len(fields) {
				return fields[i+1]
			}
		}
	}
	return "eth0"
}

type Session struct {
	ID        string
	Interface string // the real adapter this network claimed (for in-use tracking)
	SSID      string
	Hostapd   *hostapd.Process
	DNSMasq   *routing.DNSMasqProcess
	Portal    *portal.Engine
	Namespace *netns.SimNamespace
	Veth      *routing.VethTopology
}

var (
	mu      sync.Mutex
	running = map[string]*Session{}
)

// execLogged runs a command, logs it under tag, and returns combined output + error.
func execLogged(tag string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(out))
	if err != nil {
		log.Printf("[sim][%s] EXEC %s %s -> FAIL: %v | output: %s", tag, name, strings.Join(args, " "), err, outStr)
	} else {
		log.Printf("[sim][%s] EXEC %s %s -> OK | output: %s", tag, name, strings.Join(args, " "), outStr)
	}
	return outStr, err
}

// NuclearTeardown kills all Tala WTE wireless processes and purges every wte-* namespace. Explicit admin reset
// tool, not part of the normal start flow; for per-network cleanup use TargetedCleanup.
func NuclearTeardown(tag string) {
	log.Printf("[sim][%s] === NUCLEAR TEARDOWN BEGIN ===", tag)

	execLogged(tag, "pkill", "-9", "-f", "hostapd.*/tmp/hostapd-")
	execLogged(tag, "pkill", "-9", "-f", "dnsmasq.*/tmp/dnsmasq-")
	execLogged(tag, "pkill", "-9", "-f", "socat.*tala-wte")

	time.Sleep(500 * time.Millisecond)

	nsListOut, _ := execLogged(tag, "ip", "netns", "list")
	if nsListOut != "" {
		for _, line := range strings.Split(nsListOut, "\n") {
			nsEntry := strings.Fields(line)
			if len(nsEntry) == 0 {
				continue
			}
			nsName := nsEntry[0]
			if strings.HasPrefix(nsName, "wte-") {
				log.Printf("[sim][%s] Destroying stale namespace: %s", tag, nsName)
				// Quiesce the radio before returning its PHY; yanking a hot MT7921 wedges the patch semaphore.
				quiesceRadioInNetnsName(nsName)
				phyListOut, phyErr := execLogged(tag, "ip", "netns", "exec", nsName, "ls", "/sys/class/ieee80211/")
				if phyErr == nil && phyListOut != "" {
					for _, phy := range strings.Fields(phyListOut) {
						phy = strings.TrimSpace(phy)
						if phy != "" {
							log.Printf("[sim][%s] Recovering PHY %s from namespace %s", tag, phy, nsName)
							execLogged(tag, "ip", "netns", "exec", nsName, "iw", "phy", phy, "set", "netns", "1")
						}
					}
				}
				execLogged(tag, "ip", "netns", "delete", nsName)
			}
		}
	}

	// Clean up zombie veth interfaces with our naming convention.
	linkOut, _ := execLogged(tag, "ip", "link", "show")
	if linkOut != "" {
		for _, line := range strings.Split(linkOut, "\n") {
			if strings.Contains(line, "vth-") || strings.Contains(line, "veth-portal") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					ifName := strings.TrimSuffix(strings.TrimSuffix(fields[1], ":"), "@")
					if atIdx := strings.Index(ifName, "@"); atIdx > 0 {
						ifName = ifName[:atIdx]
					}
					if strings.HasPrefix(ifName, "vth-") || ifName == "veth-portal" || ifName == "veth-portal-p" {
						log.Printf("[sim][%s] Destroying zombie veth: %s", tag, ifName)
						execLogged(tag, "ip", "link", "delete", ifName)
					}
				}
			}
		}
	}

	// Delete only iptables NAT rules matching our vth-* interfaces.
	for _, chain := range []string{"PREROUTING", "POSTROUTING"} {
		deleteMatchingNATRules(tag, chain, "vth-")
	}

	// Let interfaces release and the radio settle before any network restarts (avoids a hot MT7921 re-init wedge).
	time.Sleep(radioSettleDelay())

	log.Printf("[sim][%s] === NUCLEAR TEARDOWN COMPLETE ===", tag)
}

// deleteMatchingNATRules removes only the NAT rules in chain matching pattern, never flushing the whole chain.
func deleteMatchingNATRules(tag, chain, pattern string) {
	out, _ := execLogged(tag, "iptables", "-t", "nat", "-S", chain)
	if out == "" {
		return
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "-A ") || !strings.Contains(line, pattern) {
			continue
		}
		delRule := strings.Replace(line, "-A ", "-D ", 1)
		args := append([]string{"-t", "nat"}, strings.Fields(delRule)...)
		execLogged(tag, "iptables", args...)
	}
}

// InUseInterfaces maps each adapter claimed by a running network to its SSID, so the UI can show busy adapters
// (a running PHY lives in its namespace and is absent from the host adapter list).
func InUseInterfaces() map[string]string {
	mu.Lock()
	defer mu.Unlock()
	out := make(map[string]string)
	for _, s := range running {
		if s.Interface != "" {
			out[s.Interface] = s.SSID
		}
	}
	return out
}

// radioSettleDelay is the post-teardown wait before a PHY is reused, avoiding a hot MT7921 re-init wedge.
// Defaults to 2000ms; override with TALA_RADIO_SETTLE_MS (clamped to 0..15000ms).
func radioSettleDelay() time.Duration {
	ms := 2000
	if v := strings.TrimSpace(os.Getenv("TALA_RADIO_SETTLE_MS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 15000 {
			ms = n
		}
	}
	return time.Duration(ms) * time.Millisecond
}

// quiesceRadioInNetnsName brings any wl* interface in the namespace down (then pauses) so the MT7921 isn't yanked
// from a hot state on PHY return, the trigger for the "Failed to get patch semaphore" wedge. Idempotent.
func quiesceRadioInNetnsName(nsName string) {
	out, err := exec.Command("ip", "netns", "exec", nsName, "ls", "/sys/class/net/").Output()
	if err != nil {
		return
	}
	brought := false
	for _, ifn := range strings.Fields(string(out)) {
		if strings.HasPrefix(ifn, "wl") {
			if e := exec.Command("ip", "netns", "exec", nsName, "ip", "link", "set", ifn, "down").Run(); e == nil {
				log.Printf("[sim] quiesced %s in %s before PHY return", ifn, nsName)
				brought = true
			}
		}
	}
	if brought {
		time.Sleep(300 * time.Millisecond)
	}
}

// quiesceRadioInNetns is quiesceRadioInNetnsName keyed by network ID.
func quiesceRadioInNetns(id string) { quiesceRadioInNetnsName("wte-" + id) }

// TargetedCleanup removes resources for a specific network ID without affecting other sessions.
func TargetedCleanup(id string) {
	tag := "cleanup-" + id
	nsName := fmt.Sprintf("wte-%s", id)

	log.Printf("[sim][%s] Targeted cleanup for network %s", tag, id)

	// Quiesce the radio before returning the PHY to the host, avoiding a hot MT7921 re-init wedge.
	quiesceRadioInNetnsName(nsName)

	phyListOut, phyErr := execLogged(tag, "ip", "netns", "exec", nsName, "ls", "/sys/class/ieee80211/")
	if phyErr == nil && phyListOut != "" {
		for _, phy := range strings.Fields(phyListOut) {
			phy = strings.TrimSpace(phy)
			if phy != "" {
				execLogged(tag, "ip", "netns", "exec", nsName, "iw", "phy", phy, "set", "netns", "1")
			}
		}
	}

	execLogged(tag, "ip", "netns", "delete", nsName)

	shortID := id
	if len(id) > 4 {
		shortID = id[:4]
	}
	vethHost := fmt.Sprintf("vth-%s", shortID)
	execLogged(tag, "ip", "link", "delete", vethHost)

	// Let the radio settle before the next start re-grabs the PHY (reduces the patch-semaphore wedge under cycling).
	time.Sleep(radioSettleDelay())
	log.Printf("[sim][%s] Targeted cleanup complete", tag)
}

// StartHandler starts a network by ID. WPA-Enterprise networks run CheckEnterprisePreflight first and refuse to
// start (HTTP 412) on a missing dependency unless the body sets auto_provision:true, which runs AutoProvisionEnterprise.
func StartHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		log.Printf("[sim][start] Network start requested: id=%s", id)

		record, err := app.FindRecordById("networks", id)
		if err != nil {
			log.Printf("[sim][start] Network not found: id=%s err=%v", id, err)
			api.WriteErr(w, http.StatusNotFound, "network not found")
			return
		}

		ssid := record.GetString("ssid")
		protocol := record.GetString("protocol")
		configuredIface := record.GetString("interface")

		// Read the start options once: auto_provision (enterprise) plus an optional
		// confirmed adapter/band override from the radio-management swap prompt.
		var body struct {
			AutoProvision bool   `json:"auto_provision"`
			Interface     string `json:"interface"`
			Band          string `json:"band"`
		}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}

		// Enterprise preflight + optional auto-provision before any wireless or namespace work.
		if protocol == "wpa2_enterprise" || protocol == "wpa3_enterprise" {
			pre := CheckEnterprisePreflight()
			if !pre.OK && body.AutoProvision {
				log.Printf("[sim][start] Enterprise preflight failed; auto_provision=true - running AutoProvisionEnterprise")
				if prov := AutoProvisionEnterprise(); !prov.OK {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					api.WriteJSON(w, map[string]any{
						"error":     "auto-provision failed; see steps for details",
						"provision": prov,
						"preflight": pre,
					})
					return
				}
				pre = CheckEnterprisePreflight()
			}
			if !pre.OK {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusPreconditionFailed)
				api.WriteJSON(w, map[string]any{
					"error":     "enterprise preflight failed - fix the failing checks or POST {\"auto_provision\":true} to bootstrap them automatically",
					"preflight": pre,
				})
				return
			}
		}

		// Host-visible adapters: a running network's PHY is in its namespace, so this is the free set.
		adapters := iface.DiscoverAdapters()

		mu.Lock()

		if _, exists := running[id]; exists {
			mu.Unlock()
			log.Printf("[sim][start] Network already running: id=%s", id)
			api.WriteJSON(w, map[string]any{"status": "already_running"})
			return
		}

		// Allocation guard: resolve + claim atomically under the lock so no adapter is double-claimed.
		inUse := map[string]bool{}
		for _, s := range running {
			if s.Interface != "" {
				inUse[s.Interface] = true
			}
		}

		// Radio management: if the saved adapter is physically gone and the operator
		// has not confirmed a substitute, ask before silently switching radios.
		if body.Interface == "" && configuredIface != "" && iface.FindByInterface(adapters, configuredIface) == nil {
			cand := iface.BestFreeRealAdapter(adapters, inUse)
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			if cand == nil {
				api.WriteJSON(w, map[string]any{
					"error": fmt.Sprintf("configured adapter %q is not connected and no other wireless adapter is available", configuredIface),
				})
			} else {
				api.WriteJSON(w, buildSwapProposal(configuredIface, cand, record.GetString("band")))
			}
			return
		}

		// Operator confirmed a substitute adapter (and optionally a band change): persist it.
		if body.Interface != "" {
			record.Set("interface", body.Interface)
			configuredIface = body.Interface
			if body.Band != "" && body.Band != record.GetString("band") {
				record.Set("band", body.Band)
				record.Set("channel", defaultChannelForBand(body.Band))
			}
			if err := app.Save(record); err != nil {
				mu.Unlock()
				api.WriteErr(w, http.StatusInternalServerError, "failed to save adapter change: "+err.Error())
				return
			}
		}

		ifName, substituted, resolveReason, resErr := iface.ResolveInterfaceFree(adapters, configuredIface, inUse)
		if resErr != nil {
			mu.Unlock()
			log.Printf("[sim][start] interface allocation failed for %s: %v", id, resErr)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			api.WriteJSON(w, map[string]any{"error": resErr.Error()})
			return
		}

		// Claim the adapter via a placeholder session so concurrent starts see it as in use.
		session := &Session{ID: id, Interface: ifName, SSID: ssid}
		running[id] = session
		mu.Unlock()

		if resolveReason != "" {
			log.Printf("[sim][start] Interface resolution: %s", resolveReason)
		}
		if substituted {
			log.Printf("[sim][start] Auto-substituted interface: %s -> %s", configuredIface, ifName)
		}
		log.Printf("[sim][start] Network config: ssid=%s protocol=%s interface=%s", ssid, protocol, ifName)

		// Remove the placeholder on failure.
		startFailed := true
		defer func() {
			if startFailed {
				mu.Lock()
				delete(running, id)
				mu.Unlock()
			}
		}()

		// Reject a band the resolved adapter cannot host an AP on (e.g. MT7921 won't beacon 6 GHz); guards the API path.
		if band := record.GetString("band"); band != "" {
			if a := iface.FindByInterface(adapters, ifName); a != nil {
				apBands := a.APBands
				if len(apBands) == 0 {
					apBands = a.Bands
				}
				want := map[string]string{"2.4": "2.4 GHz", "5": "5 GHz", "6": "6 GHz"}[band]
				if want != "" && len(apBands) > 0 && !slices.Contains(apBands, want) {
					log.Printf("[sim][start] band %s not AP-capable on %s (%s): supported %v", want, ifName, a.DeviceModel, apBands)
					api.WriteErr(w, http.StatusBadRequest, fmt.Sprintf(
						"%s %s cannot host an AP on %s (supported: %s)",
						a.Manufacturer, a.DeviceModel, want, strings.Join(apBands, ", "),
					))
					return
				}
			}
		}

		// Surface the resolved adapter's capability limits in the log so the operator
		// sees them when starting (e.g. a WPA3 network on a no-SAE legacy card).
		if a := iface.FindByInterface(adapters, ifName); a != nil && len(a.Limits) > 0 {
			log.Printf("[sim][start] %s limits: %s", ifName, strings.Join(a.Limits, "; "))
		}

		// Clean up any stale resources for this network only.
		TargetedCleanup(id)

		nsName := fmt.Sprintf("wte-%s", id)
		log.Printf("[sim][start] Creating namespace: %s", nsName)
		ns, err := netns.Create(nsName)
		if err != nil {
			log.Printf("[sim][start] failed to create namespace %s: %v", nsName, err)
			api.WriteErr(w, http.StatusInternalServerError, fmt.Sprintf("create ns: %v", err))
			return
		}
		session.Namespace = ns
		log.Printf("[sim][start] Namespace created: %s", nsName)

		if err := ns.SetupLoopback(); err != nil {
			log.Printf("[sim][start] failed to setup loopback in %s: %v", nsName, err)
			_ = ns.Delete() // cleanup on an error path; the original error is already returned
			api.WriteErr(w, http.StatusInternalServerError, fmt.Sprintf("setup loopback: %v", err))
			return
		}
		log.Printf("[sim][start] Loopback up in %s", nsName)

		uplinkIface := resolveUplinkIface()
		log.Printf("[sim][start] Setting up veth tunnel: id=%s ns=%s uplink=%s", id, nsName, uplinkIface)
		topology, err := routing.SetupVethTunnel(id, nsName, uplinkIface)
		if err != nil {
			log.Printf("[sim][start] failed to setup veth tunnel: %v", err)
			_ = ns.Delete() // cleanup on an error path; the original error is already returned
			api.WriteErr(w, http.StatusInternalServerError, fmt.Sprintf("setup veth tunnel: %v", err))
			return
		}
		session.Veth = topology
		log.Printf("[sim][start] Veth tunnel up: host=%s peer=%s hostIP=%s peerIP=%s", topology.HostIface, topology.PeerIface, topology.HostIP, topology.PeerIP)

		// Recover a radio left in a stale state by a prior unclean stop (e.g. stuck
		// in AP mode), which would otherwise fail to beacon. A clean idle radio
		// reads "managed" and is skipped, so normal starts are unaffected.
		if t := iface.InterfaceType(ifName); t != "" && t != "managed" {
			netLogf(id, "[sim] radio %s in stale state %q; re-binding USB to recover", ifName, t)
			if herr := iface.HealAdapter(ifName); herr != nil {
				log.Printf("[sim][start] adapter heal for %s failed: %v (continuing)", ifName, herr)
			} else {
				adapters = iface.DiscoverAdapters() // phy index can change across a rebind
			}
		}

		// Move PHY to namespace.
		phyName := ""
		log.Printf("[sim][start] Discovered %d adapters", len(adapters))
		for _, a := range adapters {
			log.Printf("[sim][start]   Adapter: interface=%s phy=%s driver=%s", a.Interface, a.Phy, a.Driver)
			if a.Interface == ifName {
				phyName = a.Phy
			}
		}

		if phyName != "" {
			log.Printf("[sim][start] Moving PHY %s (interface %s) to namespace %s", phyName, ifName, nsName)
			if err := ns.MoveInterface(phyName); err != nil {
				log.Printf("[sim][start] failed to move PHY %s: %v", phyName, err)
				routing.TeardownVethTunnel(topology)
				_ = ns.Delete() // cleanup on an error path; the original error is already returned
				api.WriteErr(w, http.StatusInternalServerError, fmt.Sprintf("failed to move phy %s to namespace: %v", phyName, err))
				return
			}
			log.Printf("[sim][start] PHY %s moved to %s", phyName, nsName)
		} else {
			log.Printf("[sim][start] No PHY found for %s, attempting raw interface move", ifName)
			if err := ns.MoveInterface(ifName); err != nil {
				log.Printf("[sim][start] failed to move interface %s: %v", ifName, err)
				routing.TeardownVethTunnel(topology)
				_ = ns.Delete() // cleanup on an error path; the original error is already returned
				api.WriteErr(w, http.StatusInternalServerError, fmt.Sprintf("failed to move hardware %s to namespace: %v", ifName, err))
				return
			}
			log.Printf("[sim][start] Interface %s moved to %s (raw fallback)", ifName, nsName)
		}

		// Start hostapd. Enterprise networks use the host-side veth IP as the RADIUS server so packets reach FreeRADIUS.
		cfg := buildConfig(record, ifName, topology.HostIP)
		confPath, err := cfg.WriteToTemp()
		if err != nil {
			log.Printf("[sim][start] failed to write hostapd config: %v", err)
			routing.TeardownVethTunnel(topology)
			_ = ns.Delete() // cleanup on an error path; the original error is already returned
			api.WriteErr(w, http.StatusInternalServerError, fmt.Sprintf("write hostapd conf: %v", err))
			return
		}
		log.Printf("[sim][start] Hostapd config written to: %s", confPath)

		proc, err := hostapd.Start(context.Background(), confPath, nsName)
		if err != nil {
			log.Printf("[sim][start] failed to start hostapd: %v", err)
			routing.TeardownVethTunnel(topology)
			_ = ns.Delete() // cleanup on an error path; the original error is already returned
			api.WriteErr(w, http.StatusInternalServerError, fmt.Sprintf("start hostapd: %v", err))
			return
		}
		session.Hostapd = proc
		log.Printf("[sim][start] Hostapd started in namespace %s", nsName)
		netLogReset(id)
		netLogf(id, "[sim] hostapd started - ssid=%q protocol=%s iface=%s", ssid, protocol, ifName)
		go drainHostapdLog(id, proc)

		// Give hostapd a second to create the interface before starting dnsmasq.
		time.Sleep(1 * time.Second)

		gwIP, dhcpStart, dhcpEnd := deriveSubnet(record.GetString("subnet"))
		if err := ns.Exec(func() error {
			return routing.AssignIP(ifName, gwIP+"/24")
		}); err != nil {
			log.Printf("[sim][start] failed to assign gateway IP: %v", err)
		}
		log.Printf("[sim][start] Gateway IP %s/24 assigned to %s inside %s", gwIP, ifName, nsName)

		// Client isolation: ap_isolate handles L2; add a same-interface FORWARD drop to also block the L3 hairpin.
		if record.GetBool("client_isolation") {
			if out, err := exec.Command("ip", "netns", "exec", nsName, "iptables",
				"-I", "FORWARD", "-i", ifName, "-o", ifName, "-j", "DROP").CombinedOutput(); err != nil {
				log.Printf("[sim][start] client isolation FORWARD drop failed: %s: %v", out, err)
			} else {
				netLogf(id, "[sim] client isolation enforced (ap_isolate + intra-AP forward drop)")
			}
		}

		if record.GetBool("portal_enabled") {
			log.Printf("[sim][start] Portal enabled, starting portal engine")
			p := portal.New(id, ifName, gwIP, dhcpStart, dhcpEnd, record.GetString("portal_html"), nsName)
			p.NetworkSSID = ssid
			// Auth portals validate credentials against the embedded directory (same store as the 802.1X path).
			p.RequireAuth = record.GetBool("portal_auth")
			p.Authenticate = ldap.Authenticate
			// Persist every harvested submission so it surfaces in the UI as captured credentials/PII.
			p.OnSubmit = func(sub portal.Submission) {
				col, cErr := app.FindCollectionByNameOrId("portal_submissions")
				if cErr != nil {
					log.Printf("[sim][portal] submissions collection unavailable: %v", cErr)
					return
				}
				rec := core.NewRecord(col)
				rec.Set("network_id", sub.NetworkID)
				rec.Set("network_ssid", sub.SSID)
				rec.Set("mac", sub.MAC)
				rec.Set("ip", sub.IP)
				rec.Set("user_agent", sub.UserAgent)
				if data, mErr := json.Marshal(sub.Fields); mErr == nil {
					rec.Set("data_json", string(data))
				}
				if sErr := app.Save(rec); sErr != nil {
					log.Printf("[sim][portal] failed to save submission: %v", sErr)
				} else {
					netLogf(id, "[portal] captured submission from %s (%d fields)", sub.MAC, len(sub.Fields))
				}
			}
			err := p.Start()
			if err == nil {
				session.Portal = p
				log.Printf("[sim][start] Portal engine started")
			} else {
				log.Printf("[sim][start] portal engine failed to start: %v", err)
			}
		} else {
			log.Printf("[sim][start] Starting dnsmasq for standard DHCP")
			dnsmasqCfg := routing.DNSMasqConfig{
				Interface: ifName,
				GatewayIP: gwIP,
				DHCPRange: dhcpStart + "," + dhcpEnd,
				SessionID: id,
			}
			if dnsmasqConfPath, err := routing.GenerateDNSMasqConfig(dnsmasqCfg); err == nil {
				dProc, dErr := routing.StartDNSMasq(dnsmasqConfPath, nsName, func(l string) { netLogAppend(id, "[dnsmasq] "+l) })
				if dErr != nil {
					log.Printf("[sim][start] dnsmasq failed to start: %v", dErr)
				} else {
					session.DNSMasq = dProc
					log.Printf("[sim][start] dnsmasq started in %s", nsName)
					netLogf(id, "[sim] dnsmasq (DHCP) started - range %s-%s, gw/dns %s", dhcpStart, dhcpEnd, gwIP)
				}
			} else {
				log.Printf("[sim][start] Failed to generate dnsmasq config: %v", err)
			}
		}

		// Enterprise networks reach FreeRADIUS via the namespace gateway; no per-network NAT plumbing required.
		if protocol == "wpa2_enterprise" || protocol == "wpa3_enterprise" {
			log.Printf("[sim][start] Enterprise mode: hostapd -> %s:1812 -> FreeRADIUS (namespace src %s)", topology.HostIP, topology.PeerIP)
		}

		startFailed = false

		record.Set("status", "running")
		if err := app.Save(record); err != nil {
			log.Printf("[sim][start] failed to persist running status for %s: %v", id, err)
		}

		log.Printf("[sim][start] === NETWORK STARTED === id=%s ssid=%s protocol=%s iface=%s ns=%s", id, ssid, protocol, ifName, nsName)
		api.WriteJSON(w, map[string]any{"status": "started", "id": id})

		go monitorSession(app, id)
	}
}

// StopHandler stops a running network.
func StopHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		log.Printf("[sim][stop] Network stop requested: id=%s", id)

		mu.Lock()
		defer mu.Unlock()

		session, exists := running[id]
		if !exists {
			log.Printf("[sim][stop] Network not in running map: id=%s -- running targeted cleanup", id)
			TargetedCleanup(id)

			record, err := app.FindRecordById("networks", id)
			if err == nil {
				record.Set("status", "stopped")
				if saveErr := app.Save(record); saveErr != nil {
					log.Printf("[sim][stop] failed to persist stopped status for %s: %v", id, saveErr)
				}
			}

			api.WriteJSON(w, map[string]any{"status": "stopped", "id": id})
			return
		}

		log.Printf("[sim][stop] Tearing down session for id=%s", id)

		if session.DNSMasq != nil {
			log.Printf("[sim][stop] Stopping dnsmasq")
			if err := session.DNSMasq.Stop(); err != nil {
				log.Printf("[sim][stop] dnsmasq stop failed: %v", err)
			}
		}
		if session.Portal != nil {
			log.Printf("[sim][stop] Stopping portal engine")
			session.Portal.Stop()
		}
		if session.Hostapd != nil {
			log.Printf("[sim][stop] Stopping hostapd")
			if err := session.Hostapd.Stop(); err != nil {
				log.Printf("[sim][stop] hostapd stop failed: %v", err)
			}
		}
		if session.Veth != nil {
			log.Printf("[sim][stop] Tearing down veth tunnel: %s", session.Veth.HostIface)
			routing.TeardownVethTunnel(session.Veth)
		}
		if session.Namespace != nil {
			log.Printf("[sim][stop] Deleting namespace")
			quiesceRadioInNetns(id)
			if err := session.Namespace.Delete(); err != nil {
				log.Printf("[sim][stop] namespace delete failed: %v", err)
			}
		}

		delete(running, id)

		record, err := app.FindRecordById("networks", id)
		if err == nil {
			record.Set("status", "stopped")
			if saveErr := app.Save(record); saveErr != nil {
				log.Printf("[sim][stop] failed to persist stopped status for %s: %v", id, saveErr)
			}
		}

		log.Printf("[sim][stop] === NETWORK STOPPED === id=%s", id)
		api.WriteJSON(w, map[string]any{"status": "stopped", "id": id})
	}
}

// StatusHandler returns the current status of a network.
func StatusHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		mu.Lock()
		session, exists := running[id]
		mu.Unlock()

		status := "stopped"
		if exists {
			if session.Hostapd != nil && session.Hostapd.IsRunning() {
				status = "running"
			} else {
				status = "error"
			}
		}

		api.WriteJSON(w, map[string]any{"id": id, "status": status})
	}
}

// ClientsHandler returns connected clients for a network.
func ClientsHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		mu.Lock()
		session, ok := running[id]
		mu.Unlock()

		if !ok || session.Hostapd == nil {
			api.WriteJSON(w, map[string]any{"clients": []any{}})
			return
		}

		clients := session.Hostapd.Clients()
		api.WriteJSON(w, map[string]any{"clients": clients})
	}
}

// LogsHandler returns the per-network live activity log for the detail page's log panel to poll.
func LogsHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		api.WriteJSON(w, map[string]any{"lines": netLogHistory(id)})
	}
}

// monitorSession watches a running hostapd process and marks the network "error" if it crashes.
func monitorSession(app *pocketbase.PocketBase, id string) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		mu.Lock()
		session, exists := running[id]
		mu.Unlock()

		if !exists {
			return
		}

		if session.Hostapd != nil && !session.Hostapd.IsRunning() {
			log.Printf("[sim][monitor] Hostapd crashed for network %s - marking as error", id)

			record, err := app.FindRecordById("networks", id)
			if err == nil {
				record.Set("status", "error")
				if saveErr := app.Save(record); saveErr != nil {
					log.Printf("[sim][monitor] failed to persist error status for %s: %v", id, saveErr)
				}
			}

			mu.Lock()
			if session.DNSMasq != nil {
				if stopErr := session.DNSMasq.Stop(); stopErr != nil {
					log.Printf("[sim][monitor] dnsmasq stop failed for %s: %v", id, stopErr)
				}
			}
			if session.Portal != nil {
				session.Portal.Stop()
			}
			if session.Veth != nil {
				routing.TeardownVethTunnel(session.Veth)
			}
			if session.Namespace != nil {
				quiesceRadioInNetns(id)
				if delErr := session.Namespace.Delete(); delErr != nil {
					log.Printf("[sim][monitor] namespace delete failed for %s: %v", id, delErr)
				}
			}
			delete(running, id)
			mu.Unlock()
			return
		}
	}
}

// StopAll stops all running sessions. Called during graceful shutdown.
func StopAll() {
	mu.Lock()
	defer mu.Unlock()

	for id, session := range running {
		log.Printf("[sim][shutdown] Stopping session: %s", id)
		if session.DNSMasq != nil {
			if err := session.DNSMasq.Stop(); err != nil {
				log.Printf("[sim][shutdown] dnsmasq stop failed for %s: %v", id, err)
			}
		}
		if session.Portal != nil {
			session.Portal.Stop()
		}
		if session.Hostapd != nil {
			if err := session.Hostapd.Stop(); err != nil {
				log.Printf("[sim][shutdown] hostapd stop failed for %s: %v", id, err)
			}
		}
		if session.Veth != nil {
			routing.TeardownVethTunnel(session.Veth)
		}
		if session.Namespace != nil {
			quiesceRadioInNetns(id)
			if err := session.Namespace.Delete(); err != nil {
				log.Printf("[sim][shutdown] namespace delete failed for %s: %v", id, err)
			}
		}
		delete(running, id)
		// Deliberately do NOT reset status to "stopped" here. StopAll runs on graceful
		// shutdown; leaving status="running" lets bootAutoStart restore the network on
		// the next boot (matching a crash, where status persists too). A deliberate
		// per-network stop goes through StopHandler, which sets "stopped".
	}
}
