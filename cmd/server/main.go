// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"

	"github.com/vtemlabs/tala-wte/internal/api"
	"github.com/vtemlabs/tala-wte/internal/certs"
	"github.com/vtemlabs/tala-wte/internal/deps"
	"github.com/vtemlabs/tala-wte/internal/ldap"
	"github.com/vtemlabs/tala-wte/internal/sim"
)

// proxyServers holds the HTTPS and HTTP proxy servers for graceful shutdown.
var proxyServers []*http.Server

func initSerialLog() {
	logFile, err := os.OpenFile("serial.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	log.SetOutput(io.MultiWriter(os.Stdout, logFile))

	// Redirect the stdout/stderr fds through a pipe so even native scheduler panics reach serial.log.
	r, w, err := os.Pipe()
	if err != nil {
		return
	}

	trueStdoutFd, err := unix.Dup(int(os.Stdout.Fd()))
	if err != nil {
		return
	}
	trueStdout := os.NewFile(uintptr(trueStdoutFd), "trueStdout")

	// Best-effort fd redirection for the serial log; if it fails we simply lose
	// the mirrored stdout/stderr, not correctness.
	_ = unix.Dup2(int(w.Fd()), int(os.Stdout.Fd()))
	_ = unix.Dup2(int(w.Fd()), int(os.Stderr.Fd()))

	go func() {
		mw := io.MultiWriter(trueStdout, logFile)
		_, _ = io.Copy(mw, r)
	}()
}

func spawnSecureProxies() {
	crtPath, keyPath, err := certs.EnsureServerCerts("tala-wte")
	if err != nil {
		log.Printf("[certs] failed to generate server SSL certificates: %v", err)
		return
	}

	targetURL, _ := url.Parse("http://127.0.0.1:8090")
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	httpsServer := &http.Server{Addr: ":8443", Handler: proxy}
	httpServer := &http.Server{Addr: ":80", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if idx := strings.Index(host, ":"); idx != -1 {
			host = host[:idx]
		}
		http.Redirect(w, r, "https://"+host+":8443"+r.RequestURI, http.StatusMovedPermanently)
	})}

	proxyServers = append(proxyServers, httpsServer, httpServer)

	go func() {
		log.Println("HTTPS Proxy Binding: 0.0.0.0:8443 -> 127.0.0.1:8090")
		if err := httpsServer.ListenAndServeTLS(crtPath, keyPath); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Fatal TLS Reverse Proxy Exception: %v", err)
		}
	}()

	go func() {
		log.Println("HTTP Port 80 Active: Enforcing strict HTTPS Redirects to :8443")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP redirect server error: %v", err)
		}
	}()
}

// shutdownProxies gracefully shuts down the HTTP/HTTPS proxy servers.
func shutdownProxies() {
	for _, srv := range proxyServers {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("[shutdown] Proxy shutdown error: %v", err)
		}
		cancel()
	}
}

func main() {
	maybeRunSubcommand()

	initSerialLog()

	// Force pb_data to a stable native path so the database is never stored on a volatile Parallels shared folder (/media/psf).
	const dataDir = "/var/lib/tala-wte"
	hasDir := false
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "--dir=") {
			hasDir = true
			break
		}
	}
	if !hasDir {
		if err := os.MkdirAll(dataDir, 0o755); err != nil {
			log.Printf("[init] could not create data dir %s: %v", dataDir, err)
		} else {
			// Insert after the subcommand (os.Args[1]) so PocketBase parses it.
			if len(os.Args) >= 2 {
				os.Args = append(os.Args[:2], append([]string{"--dir=" + dataDir}, os.Args[2:]...)...)
			}
		}
	}

	// System dependency verification runs on every boot regardless of subcommand; missing packages are installed immediately.
	if err := deps.VerifyAndInstall(); err != nil {
		log.Fatalf("[deps] failed to bootstrap system dependencies: %v", err)
	}

	if len(os.Args) >= 2 && os.Args[1] == "serve" {
		// Bind PocketBase to loopback; the TLS proxy fronts public access.
		hasHttp := false
		for _, arg := range os.Args {
			if strings.HasPrefix(arg, "--http=") {
				hasHttp = true
				break
			}
		}
		if !hasHttp {
			os.Args = append(os.Args, "--http=127.0.0.1:8090")
		}

		spawnSecureProxies()

		if err := ldap.Start(); err != nil {
			log.Printf("[ldap] failed to start embedded slapd: %v", err)
		}
	}

	app := pocketbase.New()

	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		Automigrate: true,
	})

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		// Serve the SvelteKit static build (catches / and all sub-routes).
		se.Router.GET("/{path...}", wrap(staticHandler()))

		// First-boot account setup (browser wizard; unauthenticated).
		se.Router.GET("/api/wte/setup/status", wrap(setupStatusHandler(app)))
		se.Router.POST("/api/wte/setup/complete", wrap(setupCompleteHandler(app)))
		se.Router.GET("/api/wte/license", wrap(licenseHandler()))

		// Captive-portal legal pages, so portal-preview links resolve.
		se.Router.GET("/legal/terms", wrap(legalPageHandler()))
		se.Router.GET("/legal/aup", wrap(legalPageHandler()))
		se.Router.GET("/legal/privacy", wrap(legalPageHandler()))

		// In-browser terminal (PTY over WebSocket; superuser-only, token via ?token=).
		se.Router.GET("/api/wte/terminal/ws", terminalWSHandler(app))

		se.Router.POST("/api/wte/networks/{id}/start", wrapAuth(sim.StartHandler(app)))
		se.Router.POST("/api/wte/networks/{id}/stop", wrapAuth(sim.StopHandler(app)))
		se.Router.GET("/api/wte/networks/{id}/status", wrapAuth(sim.StatusHandler(app)))
		se.Router.GET("/api/wte/networks/{id}/clients", wrapAuth(sim.ClientsHandler(app)))
		se.Router.GET("/api/wte/networks/{id}/logs", wrapAuth(sim.LogsHandler(app)))
		se.Router.GET("/api/wte/networks/{id}/client-config", wrapAuth(clientConfigExportHandler(app)))

		// Client mode: connect to another Tala WTE AP and generate traffic. These
		// accept either the member's local superuser or a den leader's agent key,
		// so the leader can drive the member remotely.
		se.Router.POST("/api/wte/client/connect", wrapAgent(app, clientConnectHandler()))
		se.Router.POST("/api/wte/client/start", wrapAgent(app, clientStartHandler()))
		se.Router.POST("/api/wte/client/stop", wrapAgent(app, clientStopHandler()))
		se.Router.POST("/api/wte/client/disconnect", wrapAgent(app, clientDisconnectHandler()))
		se.Router.GET("/api/wte/client/status", wrapAgent(app, clientStatusHandler()))
		se.Router.POST("/api/wte/client/reconnect", wrapAgent(app, clientReconnectHandler()))
		se.Router.GET("/api/wte/client/logs", wrapAgent(app, clientLogsHandler()))
		se.Router.GET("/api/wte/client/agent-key", wrapAuth(clientAgentKeyHandler(app)))
		se.Router.POST("/api/wte/client/agent-key/regenerate", wrapAuth(clientAgentKeyRegenHandler(app)))

		// Den leader: drive registered member clients.
		se.Router.POST("/api/wte/den/{id}/deploy", wrapAuth(denDeployHandler(app)))
		se.Router.POST("/api/wte/den/{id}/stop", wrapAuth(denStopHandler(app)))
		se.Router.GET("/api/wte/den/{id}/status", wrapAuth(denStatusHandler(app)))
		se.Router.POST("/api/wte/den/update", wrapAuth(denUpdateHandler(app)))

		se.Router.GET("/api/wte/enterprise/preflight", wrapAuth(sim.PreflightHandler()))
		se.Router.POST("/api/wte/enterprise/provision", wrapAuth(sim.ProvisionHandler()))

		se.Router.GET("/api/wte/portals/templates", wrapAuth(portalTemplatesHandler()))
		se.Router.POST("/api/wte/portals/upload", wrapAuth(portalUploadHandler(app)))
		se.Router.POST("/api/wte/portals/scrape", wrapAuth(portalScrapeHandler(app)))
		// Preview is unauthenticated (iframes can't attach the auth header); same-origin framed, non-sensitive content only.
		se.Router.GET("/api/wte/portals/{id}/preview", wrap(portalPreviewHandler(app)))
		se.Router.GET("/api/wte/portals/{id}/preview/{path...}", wrap(portalPreviewHandler(app)))

		se.Router.POST("/api/wte/captures/start", wrapAuth(captureStartHandler(app)))
		se.Router.POST("/api/wte/captures/{id}/stop", wrapAuth(captureStopHandler(app)))
		se.Router.GET("/api/wte/captures/{id}/download", wrapAuth(captureDownloadHandler(app)))
		se.Router.GET("/api/wte/captures/{id}/analyze", wrapAuth(captureAnalyzeHandler(app)))
		se.Router.GET("/api/wte/captures/{id}/packets", wrapAuth(capturePacketsHandler(app)))
		se.Router.GET("/api/wte/captures/{id}/packet/{n}", wrapAuth(capturePacketDetailHandler(app)))

		se.Router.POST("/api/wte/certs/ca", wrapAuth(certs.CreateCAHandler(app)))
		se.Router.POST("/api/wte/certs/server", wrapAuth(certs.CreateServerCertHandler(app)))
		se.Router.POST("/api/wte/certs/client", wrapAuth(certs.CreateClientCertHandler(app)))

		se.Router.GET("/api/wte/ldap/users", wrapAuth(ldap.ListUsersHandler(app)))
		se.Router.POST("/api/wte/ldap/users", wrapAuth(ldap.CreateUserHandler(app)))
		se.Router.PUT("/api/wte/ldap/users/{uid}", wrapAuth(ldap.UpdateUserHandler(app)))
		se.Router.DELETE("/api/wte/ldap/users/{uid}", wrapAuth(ldap.DeleteUserHandler(app)))
		se.Router.POST("/api/wte/ldap/users/{uid}/password", wrapAuth(ldap.SetPasswordHandler(app)))
		se.Router.GET("/api/wte/ldap/groups", wrapAuth(ldap.ListGroupsHandler(app)))
		se.Router.POST("/api/wte/ldap/groups", wrapAuth(ldap.CreateGroupHandler(app)))
		se.Router.DELETE("/api/wte/ldap/groups/{cn}", wrapAuth(ldap.DeleteGroupHandler(app)))
		se.Router.POST("/api/wte/ldap/groups/{cn}/members", wrapAuth(ldap.AddMemberHandler(app)))
		se.Router.DELETE("/api/wte/ldap/groups/{cn}/members/{uid}", wrapAuth(ldap.RemoveMemberHandler(app)))
		se.Router.POST("/api/wte/ldap/test-auth", wrapAuth(ldap.TestAuthHandler(app)))
		se.Router.GET("/api/wte/ldap/status", wrapAuth(ldap.StatusHandler(app)))
		se.Router.POST("/api/wte/ldap/provision", wrapAuth(ldap.ProvisionHandler(app)))
		se.Router.POST("/api/wte/ldap/provision/random", wrapAuth(ldap.RandomProvisionHandler(app)))

		se.Router.GET("/api/wte/system/interfaces", wrapAuth(interfacesHandler()))
		se.Router.GET("/api/wte/system/status", wrap(systemStatusHandler(app)))
		se.Router.POST("/api/wte/system/mode", wrapAuth(systemModeSwapHandler()))
		se.Router.GET("/api/wte/system/version", wrapAuth(versionHandler()))
		// wrapAgent so a den leader can trigger a member's self-update with its agent key.
		se.Router.POST("/api/wte/system/update", wrapAgent(app, updateHandler()))
		se.Router.GET("/api/wte/system/settings", wrapAuth(settingsGetHandler(app)))
		se.Router.POST("/api/wte/system/settings", wrapAuth(settingsSaveHandler(app)))

		se.Router.POST("/api/wte/radius/config", wrapAuth(radiusConfigHandler(app)))

		return se.Next()
	})

	// Den teardown propagation: when a network stops or is deleted, disconnect any
	// den members assigned to it so they stop chasing a network that is gone.
	app.OnRecordAfterUpdateSuccess("networks").BindFunc(func(e *core.RecordEvent) error {
		if e.Record.GetString("status") == "stopped" {
			teardownDenForNetwork(app, e.Record.Id)
		}
		return e.Next()
	})
	app.OnRecordAfterDeleteSuccess("networks").BindFunc(func(e *core.RecordEvent) error {
		teardownDenForNetwork(app, e.Record.Id)
		return e.Next()
	})

	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		if err := e.Next(); err != nil {
			return err
		}
		bootstrapCollections(app)
		// Log the one-time setup token at boot when no admin exists yet, so the
		// operator can retrieve it from the journal before opening the wizard.
		if real, _ := hasRealSuperuser(app); !real {
			ensureSetupToken(app)
		}
		// Re-export operator-configured settings so a value set in the UI survives a restart; an explicit env override wins.
		hydrateSettingEnv(app, "uplink_iface", "TALA_UPLINK_IFACE")
		hydrateSettingEnv(app, "country_code", "TALA_COUNTRY_CODE")
		return nil
	})

	app.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
		log.Println("[shutdown] Shutting down HTTPS/HTTP proxy servers...")
		shutdownProxies()
		log.Println("[shutdown] Stopping all running network sessions...")
		sim.StopAll(app)
		log.Println("[shutdown] Running nuclear teardown to clean all orphaned resources...")
		sim.NuclearTeardown("shutdown")
		log.Println("[shutdown] Stopping embedded OpenLDAP...")
		ldap.Stop()
		log.Println("[shutdown] Resetting all network statuses to stopped...")
		resetNetworkStatuses(app)
		log.Println("[shutdown] Graceful shutdown complete")
		return e.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

func wrap(h func(http.ResponseWriter, *http.Request)) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		h(e.Response, e.Request)
		return nil
	}
}

// wrapAuth restricts an /api/wte/* handler to superusers; e.Auth != nil alone would admit any authenticated record.
func wrapAuth(h func(http.ResponseWriter, *http.Request)) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if e.Auth == nil || !e.Auth.IsSuperuser() {
			api.WriteErr(e.Response, http.StatusForbidden, "superuser authentication required")
			return nil
		}
		h(e.Response, e.Request)
		return nil
	}
}
