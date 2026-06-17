// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

package main

// `tala-wte install` / `tala-wte uninstall`. install bootstraps system
// dependencies + USB wireless recovery, copies the running binary into
// /var/lib/tala-wte, writes and starts the systemd unit, and prints the URL.
// It never creates an account (first-boot admin setup is in the browser). The
// database is preserved across reinstalls, so install is also the upgrade path.

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/vtemlabs/tala-wte/internal/deps"
)

const (
	installDataDir  = "/var/lib/tala-wte"
	installUnitName = "tala-wte.service"
	installUnitFile = "/etc/systemd/system/tala-wte.service"
)

func installBinaryDest() string {
	return fmt.Sprintf("%s/tala-wte-linux-%s", installDataDir, runtime.GOARCH)
}

// maybeRunSubcommand intercepts install/uninstall (which os.Exit) before main() hands off to PocketBase; other verbs return.
func maybeRunSubcommand() {
	if len(os.Args) < 2 {
		return
	}
	switch os.Args[1] {
	case "install":
		os.Exit(runInstall(os.Args[2:]))
	case "install-client":
		os.Exit(runInstallClient(os.Args[2:]))
	case "uninstall":
		os.Exit(runUninstall(os.Args[2:]))
	}
}

// clientDepPackages are the apt packages a Tala WTE client needs to join a
// network and generate traffic.
var clientDepPackages = []string{"wpa_supplicant", "iw", "isc-dhcp-client", "iputils-ping", "ca-certificates", "wireless-regdb"}

// runInstallClient installs Tala WTE in client mode: it joins another Tala WTE AP
// from an imported config and generates traffic. Same binary + data dir + unit as
// the AP, but the unit sets TALA_MODE=client so the console shows the client view.
func runInstallClient(args []string) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Println(`Usage: tala-wte install-client

Installs Tala WTE in CLIENT mode as a systemd service (tala-wte.service). The
client joins another Tala WTE access point from an imported config and generates
traffic. Open the web UI to import a config and control traffic. Idempotent.`)
			return 0
		}
	}
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "tala-wte install-client takes no flags; received %q\n", args[0])
		return 2
	}
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "tala-wte install-client must run as root - rerun: sudo tala-wte install-client")
		return 1
	}

	fmt.Println("== Tala WTE client installer ==")
	fmt.Printf("  arch:   %s\n", runtime.GOARCH)
	fmt.Printf("  binary: %s\n", installBinaryDest())

	fmt.Println("-> installing client dependencies")
	deps.InstallPackages(clientDepPackages)
	fmt.Println("-> recovering any wedged USB Wi-Fi adapters")
	deps.HealWedgedWifiNow()

	if err := os.MkdirAll(installDataDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", installDataDir, err)
		return 1
	}
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "locate running binary: %v\n", err)
		return 1
	}
	dest := installBinaryDest()
	if err := copyBinary(self, dest); err != nil {
		fmt.Fprintf(os.Stderr, "copy %s -> %s: %v\n", self, dest, err)
		return 1
	}
	fmt.Printf("-> installed binary: %s\n", dest)

	operator := strings.TrimSpace(os.Getenv("SUDO_USER"))
	if operator == "" || operator == "root" {
		operator = firstRegularUsername()
	}
	if operator != "" {
		bootstrapETerminal(operator)
	}

	if err := os.WriteFile(installUnitFile, []byte(renderServiceUnit(dest, operator, "client")), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", installUnitFile, err)
		return 1
	}
	fmt.Printf("-> wrote unit (client mode): %s\n", installUnitFile)

	if err := runCmd("systemctl", "daemon-reload"); err != nil {
		fmt.Fprintf(os.Stderr, "daemon-reload: %v\n", err)
		return 1
	}
	_ = runCmd("systemctl", "enable", installUnitName)
	if err := runCmd("systemctl", "restart", installUnitName); err != nil {
		fmt.Fprintf(os.Stderr, "restart: %v\n", err)
		return 1
	}

	webReady := false
	for i := 0; i < 40; i++ {
		conn, derr := net.DialTimeout("tcp", "127.0.0.1:8443", time.Second)
		if derr == nil {
			_ = conn.Close()
			webReady = true
			break
		}
		time.Sleep(1 * time.Second)
	}
	host, _ := os.Hostname()
	if host == "" {
		host = "tala-wte-client"
	}
	web := "starting"
	if webReady {
		web = "ready"
	}
	line := "════════════════════════════════════════════════════════════"
	fmt.Println()
	fmt.Println(line)
	fmt.Printf("  Tala WTE client installed - web UI: %s\n", web)
	fmt.Printf("  Open the console: https://%s:8443/  (import a config, then generate traffic)\n", host)
	fmt.Println(line)
	if !webReady {
		return 1
	}
	return 0
}

func runInstall(args []string) int {
	// Scan all args for help before rejecting, so `install <junk> --help` still
	// shows usage rather than erroring on the first token.
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Println(`Usage: tala-wte install

Installs Tala WTE as a systemd service (tala-wte.service) on this host.
Takes no flags; idempotent. Re-run any time to upgrade the binary or repair
the unit. The database at /var/lib/tala-wte is preserved across reinstalls.
After install, open the web UI to create your admin account in the browser.`)
			return 0
		}
	}
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "tala-wte install takes no flags; received %q\n", args[0])
		return 2
	}

	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "tala-wte install must run as root - rerun: sudo tala-wte install")
		return 1
	}

	fmt.Println("== Tala WTE installer ==")
	fmt.Printf("  arch:   %s\n", runtime.GOARCH)
	fmt.Printf("  binary: %s\n", installBinaryDest())
	fmt.Printf("  data:   %s\n", installDataDir)
	fmt.Printf("  unit:   %s\n", installUnitFile)

	// Heavy on first run, idempotent afterwards; done here so the service starts clean.
	fmt.Println("-> verifying system dependencies")
	if err := deps.VerifyAndInstall(); err != nil {
		fmt.Fprintf(os.Stderr, "dependency bootstrap failed: %v\n", err)
		return 1
	}
	// Heal a USB Wi-Fi adapter wedged on first probe so it surfaces without a reboot.
	fmt.Println("-> recovering any wedged USB Wi-Fi adapters")
	deps.HealWedgedWifiNow()

	if err := os.MkdirAll(installDataDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", installDataDir, err)
		return 1
	}

	self, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "locate running binary: %v\n", err)
		return 1
	}
	dest := installBinaryDest()
	if err := copyBinary(self, dest); err != nil {
		fmt.Fprintf(os.Stderr, "copy %s -> %s: %v\n", self, dest, err)
		return 1
	}
	fmt.Printf("-> installed binary: %s\n", dest)

	// The operator (who ran `sudo tala-wte install`) backs the in-browser terminal; fall back to the first regular account.
	operator := strings.TrimSpace(os.Getenv("SUDO_USER"))
	if operator == "" || operator == "root" {
		operator = firstRegularUsername()
	}
	if operator != "" {
		fmt.Printf("-> terminal operator account: %s\n", operator)
		fmt.Println("-> setting up e-terminal for the operator shell")
		bootstrapETerminal(operator)
	}

	if err := os.WriteFile(installUnitFile, []byte(renderServiceUnit(dest, operator, "")), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", installUnitFile, err)
		return 1
	}
	fmt.Printf("-> wrote unit: %s\n", installUnitFile)

	if err := runCmd("systemctl", "daemon-reload"); err != nil {
		fmt.Fprintf(os.Stderr, "daemon-reload: %v\n", err)
		return 1
	}
	if err := runCmd("systemctl", "enable", installUnitName); err != nil {
		fmt.Fprintf(os.Stderr, "enable: %v\n", err)
		return 1
	}
	if err := runCmd("systemctl", "restart", installUnitName); err != nil {
		fmt.Fprintf(os.Stderr, "restart: %v\n", err)
		return 1
	}

	// Wait for systemd to mark the unit active.
	state := "unknown"
	for i := 0; i < 15; i++ {
		out, _ := exec.Command("systemctl", "is-active", installUnitName).Output()
		state = strings.TrimSpace(string(out))
		if state == "active" {
			break
		}
		time.Sleep(1 * time.Second)
	}
	// Then wait for :8443 to answer; serve starts LDAP before binding, so HTTP readiness lags systemd-active by a few seconds.
	webReady := false
	for i := 0; i < 40; i++ {
		conn, derr := net.DialTimeout("tcp", "127.0.0.1:8443", time.Second)
		if derr == nil {
			_ = conn.Close()
			webReady = true
			break
		}
		time.Sleep(1 * time.Second)
	}

	host, _ := os.Hostname()
	if host == "" {
		host = "tala-wte"
	}
	web := "starting"
	if webReady {
		web = "ready"
	}
	line := "════════════════════════════════════════════════════════════"
	fmt.Println()
	fmt.Println(line)
	fmt.Printf("  Tala WTE installed - service: %s, web UI: %s\n", state, web)
	fmt.Printf("  Open the web UI:  https://%s:8443/\n", host)
	fmt.Println("  On first visit, create your admin account in the browser.")
	fmt.Println(line)
	if state != "active" || !webReady {
		fmt.Fprintln(os.Stderr, "note: service not fully ready yet; check: journalctl -u tala-wte -n 50")
		return 1
	}
	return 0
}

func runUninstall(args []string) int {
	purge := false
	for _, a := range args {
		switch a {
		case "--purge":
			purge = true
		case "-h", "--help":
			fmt.Println(`Usage: tala-wte uninstall [--purge]

Stops and removes the tala-wte systemd service.
  --purge   also delete /var/lib/tala-wte (DATABASE AND ALL CAPTURES)`)
			return 0
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", a)
			return 2
		}
	}
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "tala-wte uninstall must run as root")
		return 1
	}
	_ = runCmd("systemctl", "disable", "--now", installUnitName)
	_ = os.Remove(installUnitFile)
	_ = runCmd("systemctl", "daemon-reload")
	fmt.Println("-> tala-wte.service removed")
	if purge {
		_ = os.RemoveAll(installDataDir)
		fmt.Printf("-> purged %s\n", installDataDir)
		// Wipe captured terminal session logs + recorder binaries so they don't survive a purge.
		wipeETerminalArtifacts()
		fmt.Println("-> wiped e-terminal session logs + recorder binaries")
	} else {
		fmt.Printf("-> preserved %s (use --purge to delete the database)\n", installDataDir)
	}
	return 0
}

func renderServiceUnit(execPath, operator, mode string) string {
	env := ""
	if operator != "" {
		env += fmt.Sprintf("Environment=TALA_OPERATOR=%s\n", operator)
	}
	desc := "Tala WTE - Wireless Training Environment"
	if mode == "client" {
		env += "Environment=TALA_MODE=client\n"
		desc = "Tala WTE - Client (traffic simulator)"
	}
	return fmt.Sprintf(`[Unit]
Description=%s
After=network-online.target tala-wte-usb3-rescan.service tala-wte-wifi-recover.service
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=%s
%sExecStart=%s serve
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
`, desc, installDataDir, env, execPath)
}

// copyBinary copies src to dest via a temp file + atomic rename, avoiding ETXTBSY when dest is the running service binary.
func copyBinary(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dest + ".new"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dest)
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
