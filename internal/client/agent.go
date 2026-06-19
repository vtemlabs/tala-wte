// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package client

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/vtemlabs/tala-wte/internal/iface"
)

// Agent owns the single client-mode session: the Wi-Fi connection, portal
// handling, and the traffic generators. One per process.
type Agent struct {
	mu          sync.Mutex
	connectMu   sync.Mutex // serializes Connect so overlapping connects (reconnect cycle + a manual/den connect) can't leave stray wpa_supplicant
	cfg         Config
	opts        TrafficOptions
	status      Status
	cancel      context.CancelFunc // stops the traffic generators
	cycleCancel context.CancelFunc // stops the reconnect cycle
	cycling     bool               // reconnect cycling active (survives Connect's status reset)
	cycles      int                // completed reconnect cycles
	logLines    []string           // ring buffer of timestamped activity for the client log window
	wpaProc     *exec.Cmd
	iface       string

	// Cached wireless-adapter health. DiscoverAdapters scans iw/sysfs and is too
	// slow to run on every status poll, so snapshot refreshes it on a TTL.
	adapterCount       int
	adapterUnsupported int
	adapterAt          time.Time
}

var agent = &Agent{status: Status{Mode: "client", PortalState: "none"}}

// Get returns the process-wide client agent.
func Get() *Agent { return agent }

func (a *Agent) snapshot() Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := a.status
	// Cycle state lives outside status so Connect's status reset does not clear it.
	s.Cycling = a.cycling
	s.Cycles = a.cycles
	s.Arch = runtime.GOARCH
	// Refresh cached adapter health on a TTL (DiscoverAdapters scans iw/sysfs and
	// is too slow to run on every poll).
	if time.Since(a.adapterAt) > 15*time.Second {
		adapters := iface.DiscoverAdapters()
		a.adapterCount = 0
		for i := range adapters {
			if !iface.IsVirtualDriver(adapters[i].Driver) {
				a.adapterCount++
			}
		}
		a.adapterUnsupported = len(iface.UnsupportedAdapters())
		a.adapterAt = time.Now()
	}
	s.Adapters = a.adapterCount
	s.AdaptersUnsupported = a.adapterUnsupported
	return s
}

// SetReconnect enables or disables reconnect cycling: while enabled, the agent
// periodically deauths and reassociates so students can capture a fresh WPA
// handshake each cycle. freq is the base interval; jitter adds a random 0..jitter
// on top of each wait. Disabling stops the cycle but keeps the connection up.
func (a *Agent) SetReconnect(enabled bool, freq, jitter time.Duration) {
	a.mu.Lock()
	if a.cycleCancel != nil {
		a.cycleCancel()
		a.cycleCancel = nil
	}
	cfg := a.cfg
	if !enabled {
		a.cycling = false
		a.cycles = 0
		a.mu.Unlock()
		a.setEvent("reconnect cycling stopped")
		return
	}
	if cfg.SSID == "" {
		a.cycling = false
		a.mu.Unlock()
		a.setEvent("connect to a network before enabling reconnect cycling")
		return
	}
	if freq < 5*time.Second {
		freq = 5 * time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cycleCancel = cancel
	a.cycling = true
	a.cycles = 0
	a.mu.Unlock()
	go a.reconnectLoop(ctx, cfg, freq, jitter)
}

// reconnectLoop waits freq (+ up to jitter), then reassociates, repeating until
// the cycle is canceled. Connect re-runs the association, producing a handshake.
func (a *Agent) reconnectLoop(ctx context.Context, cfg Config, freq, jitter time.Duration) {
	for {
		wait := freq
		if jitter > 0 {
			wait += time.Duration(rand.Int63n(int64(jitter) + 1))
		}
		a.setEvent("reconnect cycling: next deauth in %s", wait.Round(time.Second))
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		a.setEvent("reconnect cycle: deauth + reassociate (fresh handshake)")
		if err := a.Connect(cfg); err != nil {
			a.setErr("reconnect failed: %v", err)
		}
		a.mu.Lock()
		a.cycles++
		a.mu.Unlock()
	}
}

// Status returns the live client status. If we believe we are connected but the
// radio link is actually down (the AP stopped, moved, or deauthed us), mark it
// disconnected so the reported state matches reality. Skipped during a reconnect
// cycle, which manages the link itself.
func (a *Agent) Status() Status {
	a.mu.Lock()
	connected := a.status.Connected
	cycling := a.cycling
	iface := a.status.Interface
	a.mu.Unlock()
	if connected && !cycling && iface != "" && !linkUp(iface) {
		a.mu.Lock()
		dropped := a.status.Connected
		a.status.Connected = false
		a.status.IP = ""
		a.status.PortalState = "none"
		a.mu.Unlock()
		if dropped {
			a.setEvent("lost connection: access point no longer reachable")
		}
	}
	return a.snapshot()
}

// linkUp reports whether the wireless interface currently has an associated link.
func linkUp(iface string) bool {
	b, err := os.ReadFile("/sys/class/net/" + iface + "/carrier")
	return err == nil && strings.TrimSpace(string(b)) == "1"
}

func (a *Agent) setEvent(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	a.mu.Lock()
	a.status.LastEvent = msg
	a.appendLogLocked(msg)
	a.mu.Unlock()
}

func (a *Agent) setErr(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	a.mu.Lock()
	a.status.LastError = msg
	a.status.Errors++
	a.appendLogLocked("error: " + msg)
	a.mu.Unlock()
}

// clientLogCap bounds the in-memory client activity log.
const clientLogCap = 600

// appendLogLocked adds a timestamped line to the activity log. Caller holds a.mu.
func (a *Agent) appendLogLocked(line string) {
	a.logLines = append(a.logLines, time.Now().Format("15:04:05")+" "+line)
	if len(a.logLines) > clientLogCap {
		a.logLines = a.logLines[len(a.logLines)-clientLogCap:]
	}
}

// Logs returns a copy of the buffered activity log for the client log window.
func (a *Agent) Logs() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]string, len(a.logLines))
	copy(out, a.logLines)
	return out
}

// logRaw appends a single raw line (e.g. subprocess output) to the activity log.
func (a *Agent) logRaw(line string) {
	a.mu.Lock()
	a.appendLogLocked(line)
	a.mu.Unlock()
}

// agentLogWriter funnels a subprocess's stdout/stderr into the activity log so the
// Live Log window shows real terminal output (wpa_supplicant, dhclient) the same
// way the server's network log streams hostapd.
type agentLogWriter struct {
	a      *Agent
	prefix string
}

func (w *agentLogWriter) Write(p []byte) (int, error) {
	for _, line := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		if strings.TrimSpace(line) != "" {
			w.a.logRaw(w.prefix + line)
		}
	}
	return len(p), nil
}

// Connect joins the network described by cfg: it associates with wpa_supplicant,
// pulls a DHCP lease, and gets past a captive portal if one is present.
func (a *Agent) Connect(cfg Config) error {
	// Serialize connects: a reconnect cycle and a manual/den connect must not race,
	// or each would pkill then spawn its own wpa_supplicant and they would pile up.
	a.connectMu.Lock()
	defer a.connectMu.Unlock()
	a.Stop() // tear down any prior session first

	ifc, err := findWirelessInterface()
	if err != nil {
		a.setErr("no wireless interface: %v", err)
		return err
	}

	a.mu.Lock()
	a.cfg = cfg
	a.iface = ifc
	a.status = Status{Mode: "client", Interface: ifc, SSID: cfg.SSID, PortalState: "none"}
	a.mu.Unlock()

	// Clean slate on the radio: take it from any system wpa_supplicant/dhclient
	// (NetworkManager/netplan) that would otherwise fight our standalone session.
	_ = exec.Command("pkill", "-x", "wpa_supplicant").Run()
	_ = exec.Command("pkill", "-f", "dhclient").Run()
	_ = exec.Command("ip", "addr", "flush", "dev", ifc).Run()
	// Drop any leftover routes (notably a stale default via a prior AP) so they
	// cannot shadow the host's real uplink after this radio is taken over.
	_ = exec.Command("ip", "route", "flush", "dev", ifc).Run()
	_ = exec.Command("ip", "link", "set", ifc, "up").Run()
	time.Sleep(500 * time.Millisecond)

	confPath, err := writeWPAConf(cfg)
	if err != nil {
		a.setErr("wpa conf: %v", err)
		return err
	}

	a.setEvent("associating with %q", cfg.SSID)
	// -d gives verbose association/EAPOL output, streamed into the activity log so
	// the Live Log shows real terminal output like the server's hostapd -d.
	wpa := exec.Command("wpa_supplicant", "-d", "-i", ifc, "-c", confPath)
	wlw := &agentLogWriter{a: a, prefix: ""}
	wpa.Stdout = wlw
	wpa.Stderr = wlw
	if err := wpa.Start(); err != nil {
		a.setErr("wpa_supplicant: %v", err)
		return err
	}
	a.mu.Lock()
	a.wpaProc = wpa
	a.mu.Unlock()

	// Wait for association (iw link shows Connected).
	associated := false
	for i := 0; i < 30; i++ {
		out, _ := exec.Command("iw", "dev", ifc, "link").Output()
		if strings.Contains(string(out), "Connected to") {
			associated = true
			break
		}
		time.Sleep(1 * time.Second)
	}
	if !associated {
		a.setErr("failed to associate with %q", cfg.SSID)
		return fmt.Errorf("association timeout")
	}
	a.setEvent("associated; requesting DHCP lease")

	// DHCP.
	if err := a.runDHCP(ifc); err != nil {
		a.setErr("dhcp: %v", err)
		return err
	}
	ip := ifaceIP(ifc)
	gw := defaultGateway(ifc)
	a.mu.Lock()
	a.status.Connected = true
	a.status.IP = ip
	a.status.Gateway = gw
	a.mu.Unlock()
	a.setEvent("connected: ip=%s gw=%s", ip, gw)

	// Captive portal.
	if cfg.Portal.Enabled && gw != "" {
		a.handlePortal(gw, cfg.Portal)
	}
	return nil
}

// handlePortal detects a captive portal and gets past it: it submits the portal
// form (with credentials when the portal requires login, otherwise a bare accept).
func (a *Agent) handlePortal(gateway string, pc PortalConfig) {
	a.mu.Lock()
	a.status.PortalState = "detected"
	a.mu.Unlock()
	a.setEvent("captive portal: submitting accept")

	form := url.Values{}
	form.Set("accept", "1")
	if pc.Username != "" {
		form.Set("username", pc.Username)
		form.Set("password", pc.Password)
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.PostForm("http://"+gateway+"/portal/accept", form)
	if err != nil {
		a.mu.Lock()
		a.status.PortalState = "failed"
		a.mu.Unlock()
		a.setErr("portal accept: %v", err)
		return
	}
	resp.Body.Close()

	// Confirm we now have real reachability past the gateway.
	state := "passed"
	if !internetReachable() {
		state = "failed"
	}
	a.mu.Lock()
	a.status.PortalState = state
	a.mu.Unlock()
	a.setEvent("captive portal %s", state)
}

// StartTraffic launches the enabled traffic generators against the chosen scope.
func (a *Agent) StartTraffic(opts TrafficOptions) error {
	a.mu.Lock()
	if !a.status.Connected {
		a.mu.Unlock()
		return fmt.Errorf("not connected")
	}
	if a.cancel != nil {
		a.cancel() // restart with new options
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.opts = opts
	a.status.Generating = true
	gw := a.status.Gateway
	a.mu.Unlock()
	gens := []string{}
	for _, g := range []struct {
		on bool
		n  string
	}{{opts.Web, "web"}, {opts.DNS, "dns"}, {opts.Ping, "ping"}, {opts.Downloads, "downloads"}, {opts.Creds, "creds"}, {opts.Domain, "domain"}} {
		if g.on {
			gens = append(gens, g.n)
		}
	}
	scope := []string{}
	if opts.Local {
		scope = append(scope, "local")
	}
	if opts.Internet {
		scope = append(scope, "internet")
	}
	a.setEvent("traffic started: [%s] scope:[%s]", strings.Join(gens, ", "), strings.Join(scope, ", "))

	if opts.Web {
		go a.genWeb(ctx, opts, gw)
	}
	if opts.DNS {
		go a.genDNS(ctx, opts)
	}
	if opts.Ping {
		go a.genPing(ctx, opts, gw)
	}
	if opts.Downloads {
		go a.genDownloads(ctx, opts, gw)
	}
	if opts.Creds && len(opts.Credentials) > 0 {
		go a.genCreds(ctx, opts)
	}
	if opts.Domain {
		go a.genDomain(ctx, opts)
	}
	return nil
}

// StopTraffic halts the generators but keeps the connection up.
func (a *Agent) StopTraffic() {
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	a.status.Generating = false
	a.mu.Unlock()
	a.setEvent("traffic generation stopped")
}

// Stop halts traffic and tears the connection down.
func (a *Agent) Stop() {
	a.StopTraffic()
	a.mu.Lock()
	ifc := a.iface
	wpa := a.wpaProc
	a.wpaProc = nil
	a.mu.Unlock()
	if wpa != nil && wpa.Process != nil {
		_ = wpa.Process.Kill()
	}
	if ifc != "" {
		_ = exec.Command("pkill", "-f", "dhclient.*"+ifc).Run()
		_ = exec.Command("ip", "addr", "flush", "dev", ifc).Run()
		// Flush routes too, so disconnecting restores the host's real default
		// route instead of leaving a dead default via the AP we just dropped.
		_ = exec.Command("ip", "route", "flush", "dev", ifc).Run()
	}
	a.mu.Lock()
	a.status.Connected = false
	a.status.Generating = false
	a.status.PortalState = "none"
	a.mu.Unlock()
}

func (a *Agent) inc(reqs, bytes int64) {
	a.mu.Lock()
	a.status.Requests += reqs
	a.status.BytesRx += bytes
	a.mu.Unlock()
}

// ---- traffic generators -------------------------------------------------------

var browseSites = []string{
	"http://example.com/", "https://www.wikipedia.org/", "https://www.bing.com/",
	"https://www.cloudflare.com/", "https://news.ycombinator.com/", "https://www.debian.org/",
}

var lookupDomains = []string{
	"example.com", "wikipedia.org", "cloudflare.com", "github.com", "debian.org",
	"ubuntu.com", "google.com", "mozilla.org",
}

func (a *Agent) httpClient() *http.Client { return &http.Client{Timeout: 10 * time.Second} }

func (a *Agent) genWeb(ctx context.Context, opts TrafficOptions, gw string) {
	c := a.httpClient()
	// Target pool: operator URLs plus, when enabled, public browsing sites and the
	// local gateway, so custom URLs augment the mix rather than replacing the
	// internet/local browsing the toggles asked for.
	var pool []string
	pool = append(pool, opts.URLs...)
	if opts.Internet {
		pool = append(pool, browseSites...)
	}
	if opts.Local && gw != "" {
		pool = append(pool, "http://"+gw+"/")
	}
	for {
		if ctx.Err() != nil {
			return
		}
		if len(pool) == 0 {
			sleepJitter(ctx, 2*time.Second, 4*time.Second)
			continue
		}
		target := pool[rand.Intn(len(pool))]
		if resp, err := c.Get(target); err == nil {
			n, _ := io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			a.inc(1, n)
		} else {
			a.setErr("web: %v", err)
		}
		sleepJitter(ctx, 2*time.Second, 4*time.Second)
	}
}

// genCreds replays operator-supplied logins as traffic: an HTTP Basic GET and a
// form POST of username/password to each URL. Sent in cleartext over HTTP on
// purpose so the credentials are capturable for analysis/decrypt training.
func (a *Agent) genCreds(ctx context.Context, opts TrafficOptions) {
	c := a.httpClient()
	for {
		if ctx.Err() != nil {
			return
		}
		for _, cr := range opts.Credentials {
			if ctx.Err() != nil {
				return
			}
			if cr.URL == "" {
				continue
			}
			// HTTP Basic auth (credentials in the Authorization header).
			if req, err := http.NewRequestWithContext(ctx, http.MethodGet, cr.URL, nil); err == nil {
				req.SetBasicAuth(cr.Username, cr.Password)
				if resp, err := c.Do(req); err == nil {
					_, _ = io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
					a.inc(1, 0)
				} else {
					a.setErr("creds(basic): %v", err)
				}
			}
			// Form POST (credentials in the body).
			form := url.Values{"username": {cr.Username}, "password": {cr.Password}}
			if resp, err := c.PostForm(cr.URL, form); err == nil {
				_, _ = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				a.inc(1, 0)
			} else {
				a.setErr("creds(form): %v", err)
			}
		}
		sleepJitter(ctx, 5*time.Second, 12*time.Second)
	}
}

func (a *Agent) genDNS(ctx context.Context, opts TrafficOptions) {
	r := &net.Resolver{}
	// Augment operator domains with the default lookup set when Internet is on, so
	// custom domains add to the mix rather than replacing it.
	var domains []string
	domains = append(domains, opts.Domains...)
	if opts.Internet {
		domains = append(domains, lookupDomains...)
	}
	if len(domains) == 0 {
		domains = lookupDomains
	}
	for {
		if ctx.Err() != nil {
			return
		}
		d := domains[rand.Intn(len(domains))]
		if _, err := r.LookupHost(ctx, d); err == nil {
			a.inc(1, 0)
		} else {
			a.setErr("dns: %v", err)
		}
		sleepJitter(ctx, 1*time.Second, 3*time.Second)
	}
}

func (a *Agent) genPing(ctx context.Context, opts TrafficOptions, gw string) {
	for {
		if ctx.Err() != nil {
			return
		}
		targets := []string{}
		targets = append(targets, opts.IPs...) // operator-supplied hosts
		if opts.Local && gw != "" {
			targets = append(targets, gw, localSweepTarget(gw))
		}
		if opts.Internet {
			targets = append(targets, "1.1.1.1", "8.8.8.8")
		}
		for _, t := range targets {
			if exec.CommandContext(ctx, "ping", "-c", "1", "-W", "2", t).Run() == nil {
				a.inc(1, 0)
			}
		}
		sleepJitter(ctx, 2*time.Second, 5*time.Second)
	}
}

func (a *Agent) genDownloads(ctx context.Context, opts TrafficOptions, gw string) {
	c := &http.Client{Timeout: 30 * time.Second}
	for {
		if ctx.Err() != nil {
			return
		}
		var target string
		if opts.Internet {
			target = "https://speed.cloudflare.com/__down?bytes=2000000"
		} else if opts.Local && gw != "" {
			target = "http://" + gw + "/"
		}
		if target != "" {
			if resp, err := c.Get(target); err == nil {
				n, _ := io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				a.inc(1, n)
			} else {
				a.setErr("download: %v", err)
			}
		}
		sleepJitter(ctx, 8*time.Second, 15*time.Second)
	}
}

// ---- helpers ------------------------------------------------------------------

func sleepJitter(ctx context.Context, min, max time.Duration) {
	d := min + time.Duration(rand.Int63n(int64(max-min)+1))
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// localSweepTarget returns a pseudo-random host in the gateway's /24 to create
// intra-LAN chatter (most will not answer, which is itself realistic).
func localSweepTarget(gw string) string {
	parts := strings.Split(gw, ".")
	if len(parts) != 4 {
		return gw
	}
	return fmt.Sprintf("%s.%s.%s.%d", parts[0], parts[1], parts[2], 2+rand.Intn(250))
}

func internetReachable() bool {
	c := &http.Client{Timeout: 6 * time.Second}
	resp, err := c.Get("https://1.1.1.1/")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

func findWirelessInterface() (string, error) {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if _, err := os.Stat("/sys/class/net/" + e.Name() + "/wireless"); err == nil {
			return e.Name(), nil
		}
	}
	return "", fmt.Errorf("none found")
}

func writeWPAConf(cfg Config) (string, error) {
	var nb strings.Builder
	nb.WriteString("network={\n")
	fmt.Fprintf(&nb, "\tssid=\"%s\"\n", confEscape(cfg.SSID))
	if cfg.Hidden {
		nb.WriteString("\tscan_ssid=1\n")
	}
	switch cfg.Protocol {
	case "open", "wep":
		nb.WriteString("\tkey_mgmt=NONE\n")
		if cfg.Protocol == "wep" && cfg.Passphrase != "" {
			fmt.Fprintf(&nb, "\twep_key0=\"%s\"\n\twep_tx_keyidx=0\n", confEscape(cfg.Passphrase))
		}
	case "wpa3":
		nb.WriteString("\tkey_mgmt=SAE\n\tieee80211w=2\n")
		fmt.Fprintf(&nb, "\tsae_password=\"%s\"\n", confEscape(cfg.Passphrase))
	case "wpa3_transition":
		nb.WriteString("\tkey_mgmt=SAE WPA-PSK\n\tieee80211w=1\n")
		fmt.Fprintf(&nb, "\tpsk=\"%s\"\n", confEscape(cfg.Passphrase))
	case "wpa2_enterprise", "wpa3_enterprise":
		nb.WriteString("\tkey_mgmt=WPA-EAP\n\teap=PEAP\n\tphase2=\"auth=MSCHAPV2\"\n")
		if cfg.Protocol == "wpa3_enterprise" {
			nb.WriteString("\tieee80211w=2\n")
		}
		fmt.Fprintf(&nb, "\tidentity=\"%s\"\n\tpassword=\"%s\"\n", confEscape(cfg.Identity), confEscape(cfg.EAPPassword))
	default: // wpa, wpa2, wps
		fmt.Fprintf(&nb, "\tpsk=\"%s\"\n", confEscape(cfg.Passphrase))
	}
	nb.WriteString("}\n")

	content := "ctrl_interface=/run/wpa_supplicant\n" + nb.String()
	path := "/tmp/wpa-tala-client.conf"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func confEscape(s string) string {
	return strings.NewReplacer("\"", "", "\n", "", "\r", "").Replace(s)
}

// runDHCP pulls a lease, retrying because a single attempt right after
// association often fails (exit 1) before the link has settled enough to pass
// DHCP. Each attempt is hard-bounded so a stuck client never wedges connect.
func (a *Agent) runDHCP(ifc string) error {
	attempt := func(name string, args ...string) error {
		a.logRaw("dhcp: running " + name + " " + strings.Join(args, " "))
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
		for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
			if strings.TrimSpace(line) != "" {
				a.logRaw("dhclient: " + line)
			}
		}
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if len(msg) > 200 {
				msg = msg[len(msg)-200:]
			}
			return fmt.Errorf("%w: %s", err, msg)
		}
		return nil
	}
	var run func() error
	switch {
	case lookPath("dhclient"):
		run = func() error { return attempt("dhclient", "-1", ifc) }
	case lookPath("dhcpcd"):
		run = func() error { return attempt("dhcpcd", "-t", "18", ifc) }
	case lookPath("udhcpc"):
		run = func() error { return attempt("udhcpc", "-i", ifc, "-n", "-q") }
	default:
		return fmt.Errorf("no DHCP client (dhclient/dhcpcd/udhcpc) found")
	}

	var lastErr error
	for i := 0; i < 6; i++ {
		if lastErr = run(); lastErr == nil {
			if ifaceIP(ifc) != "" {
				return nil
			}
			lastErr = fmt.Errorf("dhcp succeeded but no address bound")
		}
		time.Sleep(3 * time.Second) // let the link settle, then retry
	}
	return lastErr
}

func lookPath(p string) bool {
	_, err := exec.LookPath(p)
	return err == nil
}

func ifaceIP(ifc string) string {
	out, err := exec.Command("ip", "-4", "-o", "addr", "show", "dev", ifc).Output()
	if err != nil {
		return ""
	}
	for _, f := range strings.Fields(string(out)) {
		if strings.Contains(f, ".") && strings.Contains(f, "/") {
			return strings.Split(f, "/")[0]
		}
	}
	return ""
}

func defaultGateway(ifc string) string {
	out, err := exec.Command("ip", "route", "show", "dev", ifc).Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "default via ") {
			f := strings.Fields(line)
			if len(f) >= 3 {
				return f[2]
			}
		}
	}
	return ""
}
