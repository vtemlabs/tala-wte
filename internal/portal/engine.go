// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

// Package portal implements the captive portal engine: dnsmasq, iptables HTTP redirect, MAC allowlist, and portal HTTP server.
package portal

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/vishvananda/netns"
)

const (
	portalListenAddr = ":8080"
	portalGatewayIP  = "10.0.0.1"
	dhcpRangeStart   = "10.0.0.10"
	dhcpRangeEnd     = "10.0.0.250"
)

// Submission is a single captured set of credentials/PII entered into a portal form.
type Submission struct {
	NetworkID string            `json:"network_id"`
	SSID      string            `json:"ssid"`
	MAC       string            `json:"mac"`
	IP        string            `json:"ip"`
	UserAgent string            `json:"user_agent"`
	Fields    map[string]string `json:"fields"`
	Time      time.Time         `json:"time"`
}

// Engine manages a captive portal for a single open network.
type Engine struct {
	NetworkID   string
	NetworkSSID string
	Interface   string
	GatewayIP   string
	DHCPStart   string
	DHCPEnd     string
	PortalHTML  string
	NetnsName   string

	// OnSubmit, if set, is invoked with every portal form submission so the caller can persist harvested fields.
	OnSubmit func(Submission)

	// RequireAuth makes the portal validate submitted credentials before granting access.
	RequireAuth bool

	// Authenticate validates a username/password and returns true if accepted.
	Authenticate func(username, password string) bool

	mu        sync.RWMutex
	allowed   map[string]bool // MAC -> allowed
	server    *http.Server
	dnsmasq   *exec.Cmd
	dnsCancel context.CancelFunc
}

// New creates a new captive portal engine for the given network/interface. An
// empty gateway/DHCP range falls back to the historical 10.0.0.0/24 defaults.
func New(networkID, iface, gatewayIP, dhcpStart, dhcpEnd, portalHTML, netnsName string) *Engine {
	if gatewayIP == "" {
		gatewayIP = portalGatewayIP
	}
	if dhcpStart == "" {
		dhcpStart = dhcpRangeStart
	}
	if dhcpEnd == "" {
		dhcpEnd = dhcpRangeEnd
	}
	return &Engine{
		NetworkID:  networkID,
		Interface:  iface,
		GatewayIP:  gatewayIP,
		DHCPStart:  dhcpStart,
		DHCPEnd:    dhcpEnd,
		PortalHTML: portalHTML,
		NetnsName:  netnsName,
		allowed:    make(map[string]bool),
	}
}

// Start launches dnsmasq and the portal HTTP server.
func (e *Engine) Start() error {
	// Normalize inline markup so any imported portal posts to /portal/accept; fs: bundles are normalized at extract time.
	if e.PortalHTML != "" && !strings.HasPrefix(e.PortalHTML, "fs:") {
		e.PortalHTML = Normalize(e.PortalHTML)
	}
	if err := e.startDNSMasq(); err != nil {
		return fmt.Errorf("dnsmasq: %w", err)
	}
	if err := e.addIPTablesRules(); err != nil {
		return fmt.Errorf("iptables: %w", err)
	}
	if err := e.startHTTPServer(); err != nil {
		return fmt.Errorf("portal http: %w", err)
	}
	return nil
}

// Stop tears down the portal engine.
func (e *Engine) Stop() {
	if e.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		if err := e.server.Shutdown(ctx); err != nil {
			log.Printf("[portal] http server shutdown error: %v", err)
		}
		cancel()
	}
	if e.dnsCancel != nil {
		e.dnsCancel()
	}
	e.removeIPTablesRules()
}

// AllowMAC adds a MAC to the allowlist and punches the iptables hole.
func (e *Engine) AllowMAC(mac string) error {
	mac = strings.ToLower(mac)
	e.mu.Lock()
	e.allowed[mac] = true
	e.mu.Unlock()
	// RETURN before the DNAT rule so an authenticated client's port-80 traffic is no longer redirected to the portal.
	if err := e.iptables("-t", "nat", "-I", "PREROUTING", "1", "-i", e.Interface,
		"-p", "tcp", "--dport", "80", "-m", "mac", "--mac-source", mac, "-j", "RETURN"); err != nil {
		return err
	}
	return e.iptables("-I", "FORWARD", "1", "-i", e.Interface,
		"-m", "mac", "--mac-source", mac, "-j", "ACCEPT")
}

// IsAllowed returns whether a MAC has been granted access.
func (e *Engine) IsAllowed(mac string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.allowed[strings.ToLower(mac)]
}

func (e *Engine) startDNSMasq() error {
	// Resolve real DNS; the portal is enforced by the port-80 DNAT below, not by DNS, so authenticated clients can reach the internet.
	confContent := fmt.Sprintf(`interface=%s
dhcp-range=%s,%s,12h
dhcp-option=3,%s
dhcp-option=6,%s
no-resolv
server=8.8.8.8
server=1.1.1.1
log-dhcp
`, e.Interface, e.DHCPStart, e.DHCPEnd, e.GatewayIP, e.GatewayIP)

	confPath := filepath.Join(os.TempDir(), "dnsmasq-"+e.NetworkID+".conf")
	if err := os.WriteFile(confPath, []byte(confContent), 0o600); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	e.dnsCancel = cancel

	args := []string{}
	if e.NetnsName != "" {
		args = append(args, "netns", "exec", e.NetnsName)
	}
	args = append(args, "dnsmasq",
		"--no-daemon",
		"--conf-file="+confPath,
		"--pid-file="+os.TempDir()+"/dnsmasq-"+e.NetworkID+".pid",
	)

	e.dnsmasq = exec.CommandContext(ctx, "ip", args...)
	if err := e.dnsmasq.Start(); err != nil {
		return err
	}
	// Reap the process on exit so a stopped portal does not leave a defunct dnsmasq zombie.
	go func() { _ = e.dnsmasq.Wait() }()
	return nil
}

func (e *Engine) addIPTablesRules() error {
	if err := e.iptables("-t", "nat", "-A", "PREROUTING",
		"-i", e.Interface, "-p", "tcp", "--dport", "80",
		"-j", "DNAT", "--to", e.GatewayIP+":8080"); err != nil {
		return err
	}
	return e.iptables("-A", "FORWARD", "-i", e.Interface, "-j", "DROP")
}

func (e *Engine) removeIPTablesRules() {
	_ = e.iptables("-t", "nat", "-D", "PREROUTING",
		"-i", e.Interface, "-p", "tcp", "--dport", "80",
		"-j", "DNAT", "--to", e.GatewayIP+":8080")
	_ = e.iptables("-D", "FORWARD", "-i", e.Interface, "-j", "DROP")
}

func (e *Engine) startHTTPServer() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/portal/accept", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		clientIP := strings.Split(r.RemoteAddr, ":")[0]
		mac := e.getMACFromIP(clientIP)

		// Harvest every submitted field except control fields.
		_ = r.ParseForm()
		fields := make(map[string]string)
		for k, vals := range r.PostForm {
			if k == "redirect" || len(vals) == 0 {
				continue
			}
			fields[k] = vals[0]
		}

		// Auth portals validate credentials before granting access; the result is recorded with the submission.
		authPassed := true
		if e.RequireAuth {
			user, pass := extractCreds(r.PostForm)
			authPassed = user != "" && pass != "" && e.Authenticate != nil && e.Authenticate(user, pass)
			if user != "" {
				fields["_auth_user"] = user
			}
			if authPassed {
				fields["_auth_result"] = "success"
			} else {
				fields["_auth_result"] = "fail"
			}
		}

		if e.OnSubmit != nil && len(fields) > 0 {
			e.OnSubmit(Submission{
				NetworkID: e.NetworkID,
				SSID:      e.NetworkSSID,
				MAC:       mac,
				IP:        clientIP,
				UserAgent: r.UserAgent(),
				Fields:    fields,
				Time:      time.Now(),
			})
		}

		// Reject failed logins: re-serve the portal with an error banner and do not punch the firewall hole.
		if e.RequireAuth && !authPassed {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			e.servePortalBody(w, "Authentication failed. Check your username and password and try again.")
			return
		}

		if mac != "" {
			_ = e.AllowMAC(mac)
		}
		// Restrict the reflected redirect to http/https/relative so it can't be a javascript:/data: URI.
		redirect := r.FormValue("redirect")
		if !isSafeRedirect(redirect) {
			redirect = "http://connectivitycheck.gstatic.com/generate_204"
		}
		http.Redirect(w, r, redirect, http.StatusFound)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(e.PortalHTML, "fs:") {
			baseDir := filepath.Join("/var/lib/tala-wte/portals", strings.TrimPrefix(e.PortalHTML, "fs:"))
			requestedPath := filepath.Clean(r.URL.Path)
			target := filepath.Join(baseDir, requestedPath)

			// Prevent path traversal: target must be baseDir or sit below it on a path-separator boundary.
			cleanTarget := filepath.Clean(target)
			cleanBase := filepath.Clean(baseDir)
			if cleanTarget != cleanBase && !strings.HasPrefix(cleanTarget, cleanBase+string(os.PathSeparator)) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			if stat, err := os.Stat(target); err == nil && !stat.IsDir() {
				http.ServeFile(w, r, target)
				return
			}
			http.ServeFile(w, r, filepath.Join(baseDir, "index.html"))
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if e.PortalHTML != "" {
			fmt.Fprint(w, e.PortalHTML)
		} else {
			renderDefaultPortal(w)
		}
	})

	// Legal pages registered before the "/" catch-all so the links portal templates render resolve.
	for path, doc := range legalPages {
		page := legalPageHTML(doc)
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, page)
		})
	}

	// OS captive-portal detection endpoints.
	for _, path := range []string{
		"/generate_204",
		"/hotspot-detect.html",
		"/ncsi.txt",
		"/connecttest.txt",
		"/redirect",
		"/success.txt",
	} {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://"+e.GatewayIP+"/", http.StatusFound)
		})
	}

	e.server = &http.Server{
		Addr:    e.GatewayIP + portalListenAddr,
		Handler: mux,
	}

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		if e.NetnsName != "" {
			originNS, err := netns.Get()
			if err != nil {
				log.Printf("[portal] Failed to get origin namespace: %v", err)
				return
			}
			defer func() { _ = originNS.Close() }()

			targetNS, err := netns.GetFromName(e.NetnsName)
			if err != nil {
				log.Printf("[portal] Failed to get namespace %s: %v", e.NetnsName, err)
				return
			}
			defer func() { _ = targetNS.Close() }()

			if err := netns.Set(targetNS); err != nil {
				log.Printf("[portal] Failed to enter namespace %s: %v", e.NetnsName, err)
				return
			}
			defer netns.Set(originNS)
		}

		if err := e.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[portal] HTTP server error: %v", err)
		}
	}()

	return nil
}

func renderDefaultPortal(w http.ResponseWriter) {
	tmpl := template.Must(template.New("portal").Parse(defaultPortalHTML))
	_ = tmpl.Execute(w, nil)
}

// isSafeRedirect allows only an http/https absolute URL or a site-relative path.
func isSafeRedirect(s string) bool {
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "//") {
		return true
	}
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https")
}

// credUserKeys and credPassKeys are the form field names treated as username and password.
var (
	credUserKeys = []string{
		"username", "user", "uid", "login", "account", "email",
		"member_id", "memberid", "loyalty_id", "employee_id", "guest_id",
	}
	credPassKeys = []string{"password", "pass", "pwd", "passcode", "pin"}
)

// extractCreds pulls the first username-like and password-like field out of a submitted form.
func extractCreds(form url.Values) (user, pass string) {
	for _, k := range credUserKeys {
		if v := strings.TrimSpace(form.Get(k)); v != "" {
			user = v
			break
		}
	}
	for _, k := range credPassKeys {
		if v := form.Get(k); v != "" {
			pass = v
			break
		}
	}
	return user, pass
}

// servePortalBody renders the configured portal page, optionally with an error banner injected at the top.
func (e *Engine) servePortalBody(w http.ResponseWriter, errMsg string) {
	var html string
	switch {
	case strings.HasPrefix(e.PortalHTML, "fs:"):
		baseDir := filepath.Join("/var/lib/tala-wte/portals", strings.TrimPrefix(e.PortalHTML, "fs:"))
		if b, err := os.ReadFile(filepath.Join(baseDir, "index.html")); err == nil {
			html = string(b)
		}
	case e.PortalHTML != "":
		html = e.PortalHTML
	}
	if html == "" {
		html = defaultPortalHTML
	}
	if errMsg != "" {
		html = injectError(html, errMsg)
	}
	fmt.Fprint(w, html)
}

// injectError inserts a visible error banner just after the opening <body> tag, or at the top if there is none.
func injectError(html, msg string) string {
	banner := `<div style="background:#dc2626;color:#fff;padding:12px 16px;text-align:center;` +
		`font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;font-size:14px;` +
		`position:sticky;top:0;z-index:99999">` + template.HTMLEscapeString(msg) + `</div>`
	lower := strings.ToLower(html)
	if i := strings.Index(lower, "<body"); i >= 0 {
		if j := strings.IndexByte(lower[i:], '>'); j >= 0 {
			pos := i + j + 1
			return html[:pos] + banner + html[pos:]
		}
	}
	return banner + html
}

func (e *Engine) getMACFromIP(ip string) string {
	var cmd *exec.Cmd
	if e.NetnsName != "" {
		cmd = exec.Command("ip", "netns", "exec", e.NetnsName, "ip", "neigh", "show", ip)
	} else {
		cmd = exec.Command("ip", "neigh", "show", ip)
	}

	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	// "192.168.1.5 dev wlan0 lladdr aa:bb:cc:dd:ee:ff REACHABLE"
	fields := strings.Fields(string(out))
	for i, f := range fields {
		if f == "lladdr" && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return ""
}

func (e *Engine) iptables(args ...string) error {
	var cmd *exec.Cmd
	if e.NetnsName != "" {
		fullArgs := append([]string{"netns", "exec", e.NetnsName, "iptables"}, args...)
		cmd = exec.Command("ip", fullArgs...)
	} else {
		cmd = exec.Command("iptables", args...)
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("iptables %v: %s: %w", args, out, err)
	}
	return nil
}

var defaultPortalHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Network Access</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
  background: #f5f5f5; display: flex; align-items: center; justify-content: center;
  min-height: 100vh; margin: 0; }
.card { background: white; border-radius: 12px; padding: 2rem; max-width: 400px;
  width: 90%; box-shadow: 0 4px 24px rgba(0,0,0,0.1); text-align: center; }
h1 { font-size: 1.5rem; color: #1a1a1a; margin-bottom: 0.5rem; }
p { color: #666; font-size: 0.9rem; margin-bottom: 1.5rem; }
button { background: #2563eb; color: white; border: none; border-radius: 8px;
  padding: 0.75rem 2rem; font-size: 1rem; cursor: pointer; width: 100%; }
button:hover { background: #1d4ed8; }
</style>
</head>
<body>
<div class="card">
  <h1>Welcome</h1>
  <p>Click below to accept the terms and connect to the network.</p>
  <form method="POST" action="/portal/accept">
    <input type="hidden" name="redirect" value="http://example.com">
    <button type="submit">Connect to Internet</button>
  </form>
</div>
</body>
</html>`
