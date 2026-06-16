// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

// Package ldap manages the embedded OpenLDAP (slapd) instance.
package ldap

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase"

	"github.com/vtemlabs/tala-wte/internal/api"
)

const (
	ldapDataDir       = "/var/lib/tala-wte/ldap"
	ldapConfFile      = "/var/lib/tala-wte/ldap/slapd.conf"
	ldapHost          = "ldap://127.0.0.1:3389"
	defaultBaseDN     = "dc=tala,dc=wte"
	defaultBindDN     = "cn=admin,dc=tala,dc=wte"
	adminPasswordFile = "/var/lib/tala-wte/ldap/.admin_password"
)

var (
	slapdMu   sync.Mutex
	slapdProc *exec.Cmd
	slapdCtx  context.CancelFunc
)

// Start launches slapd if not already running.
func Start() error {
	slapdMu.Lock()
	defer slapdMu.Unlock()

	dbDir := filepath.Join(ldapDataDir, "db")

	if err := os.MkdirAll(dbDir, 0o750); err != nil {
		return fmt.Errorf("mkdir ldap db: %w", err)
	}

	dbEmpty := true
	entries, _ := os.ReadDir(dbDir)
	for _, e := range entries {
		if !e.IsDir() && e.Name() != ".gitkeep" {
			dbEmpty = false
			break
		}
	}

	// Kill any stale slapd on our port from a previous run.
	_ = exec.Command("fuser", "-k", "3389/tcp").Run()
	time.Sleep(200 * time.Millisecond)

	// Always regenerate config so password and paths stay correct.
	if err := writeDefaultConfig(); err != nil {
		return fmt.Errorf("write slapd config: %w", err)
	}

	if dbEmpty {
		log.Printf("[ldap] Empty database detected - bootstrapping directory with ACME Corp users")
		if err := bootstrapDirectory(); err != nil {
			return fmt.Errorf("bootstrap ldap: %w", err)
		}
		log.Printf("[ldap] Bootstrap complete: 15 users, 2 groups created")
	}

	ctx, cancel := context.WithCancel(context.Background())
	// TCP only: an unescaped ldapi:// socket path made slapd fail to bind any listener.
	cmd := exec.CommandContext(ctx, "slapd",
		"-f", ldapConfFile,
		"-h", ldapHost+"/",
		"-d", "256",
	)
	// Capture slapd's debug output so a refused start shows why.
	var slapdLog strings.Builder
	cmd.Stdout = &slapdLog
	cmd.Stderr = &slapdLog
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("slapd start: %w", err)
	}

	slapdProc = cmd
	slapdCtx = cancel

	// Wait for slapd to start, then verify it accepts connections.
	for range 5 {
		time.Sleep(300 * time.Millisecond)
		if cmd.ProcessState != nil {
			cancel()
			return fmt.Errorf("[ldap] slapd exited immediately: %s", strings.TrimSpace(slapdLog.String()))
		}
		probe := exec.Command("ldapsearch", "-x", "-H", ldapHost, "-b", "", "-s", "base", "(objectClass=*)")
		if err := probe.Run(); err == nil {
			log.Printf("[ldap] slapd started and accepting connections on %s (base: %s)", ldapHost, defaultBaseDN)
			return nil
		}
	}

	cancel()
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	return fmt.Errorf("[ldap] slapd started but never accepted connections; slapd output: %s", strings.TrimSpace(slapdLog.String()))
}

// Stop shuts down slapd.
func Stop() {
	slapdMu.Lock()
	defer slapdMu.Unlock()
	if slapdCtx != nil {
		slapdCtx()
		slapdCtx = nil
	}
	if slapdProc != nil && slapdProc.Process != nil {
		_ = slapdProc.Wait()
		slapdProc = nil
	}
}

// IsRunning returns whether slapd is currently running.
func IsRunning() bool {
	slapdMu.Lock()
	defer slapdMu.Unlock()
	if slapdProc == nil {
		return false
	}
	return slapdProc.Process != nil && slapdProc.ProcessState == nil
}

// AdminPassword returns the LDAP admin password, preferring the
// TALA_LDAP_ADMIN_PASSWORD env var, then the persisted file, then a new random one.
func AdminPassword() string {
	if pwd := os.Getenv("TALA_LDAP_ADMIN_PASSWORD"); pwd != "" {
		return pwd
	}

	if data, err := os.ReadFile(adminPasswordFile); err == nil && len(data) > 0 {
		return strings.TrimSpace(string(data))
	}

	generated, err := generateAdminPassword(24)
	if err != nil {
		log.Fatalf("[ldap] FATAL: failed to generate random admin password: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(adminPasswordFile), 0o750); err == nil {
		if err := os.WriteFile(adminPasswordFile, []byte(generated), 0o600); err != nil {
			log.Printf("[ldap] failed to persist admin password to %s: %v", adminPasswordFile, err)
		}
	}
	log.Printf("[ldap] admin password generated and persisted to %s", adminPasswordFile)
	return generated
}

func generateAdminPassword(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		b[i] = charset[n.Int64()]
	}
	return string(b), nil
}

func ldapadd(ldif string) error {
	cmd := exec.Command("ldapadd",
		"-x", "-H", ldapHost,
		"-D", defaultBindDN,
		"-w", AdminPassword(),
	)
	cmd.Stdin = strings.NewReader(ldif)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ldapadd: %s: %w", out, err)
	}
	return nil
}

func ldapmodify(ldif string) error {
	cmd := exec.Command("ldapmodify",
		"-x", "-H", ldapHost,
		"-D", defaultBindDN,
		"-w", AdminPassword(),
	)
	cmd.Stdin = strings.NewReader(ldif)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ldapmodify: %s: %w", out, err)
	}
	return nil
}

func ldapdelete(dn string) error {
	out, err := exec.Command("ldapdelete",
		"-x", "-H", ldapHost,
		"-D", defaultBindDN,
		"-w", AdminPassword(),
		dn,
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ldapdelete %s: %s: %w", dn, out, err)
	}
	return nil
}

func ldapsearch(filter, base string) ([]map[string]string, error) {
	out, err := exec.Command("ldapsearch",
		"-x", "-LLL",
		"-H", ldapHost,
		"-D", defaultBindDN,
		"-w", AdminPassword(),
		"-b", base,
		filter,
	).Output()
	if err != nil {
		return nil, fmt.Errorf("ldapsearch: %w", err)
	}
	return parseLDIF(string(out)), nil
}

// StatusHandler returns slapd status.
func StatusHandler(app *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		running := IsRunning()
		api.WriteJSON(w, map[string]any{"running": running, "base_dn": defaultBaseDN})
	}
}
