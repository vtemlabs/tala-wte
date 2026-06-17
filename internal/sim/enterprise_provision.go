// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

package sim

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/vtemlabs/tala-wte/internal/certs"
	"github.com/vtemlabs/tala-wte/internal/ldap"
)

func lastConfigError(out []byte) string {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if l == "" {
			continue
		}
		if strings.Contains(strings.ToLower(l), "error") || strings.Contains(l, "/etc/freeradius") {
			return l
		}
	}
	if n := len(lines); n > 0 {
		return lines[n-1]
	}
	return "unknown error"
}

const (
	radiusCertDir = "/etc/freeradius/3.0/certs"
)

// EnterpriseProvisionResult reports what AutoProvisionEnterprise did, per step.
type EnterpriseProvisionResult struct {
	OK    bool                      `json:"ok"`
	Steps []EnterpriseProvisionStep `json:"steps"`
	Users []ldap.ProvisionUser      `json:"users,omitempty"` // populated when LDAP was provisioned
}

// EnterpriseProvisionStep is a single line in the auto-provision report.
type EnterpriseProvisionStep struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Status string `json:"status"` // "created" | "skipped" | "failed"
	Detail string `json:"detail,omitempty"`
}

// AutoProvisionEnterprise brings every enterprise dependency to a known-good state in dependency order.
// Idempotent: each step skips itself when its invariant already holds. Always returns the per-step report.
func AutoProvisionEnterprise() *EnterpriseProvisionResult {
	res := &EnterpriseProvisionResult{OK: true}

	// 1. Certificate Authority - required to sign the FreeRADIUS server cert below.
	if certs.CAExists() {
		res.add("ca", "Certificate Authority", "skipped", "already initialized")
	} else if err := certs.EnsureCA(); err != nil {
		res.fail("ca", "Certificate Authority", err)
	} else {
		res.add("ca", "Certificate Authority", "created", "issued 10-year CA at "+certs.CADir())
	}

	// 2. FreeRADIUS server certificate - required for the EAP-PEAP TLS tunnel.
	if certs.CertExists(certs.ServerCertName) {
		res.add("server_cert", "FreeRADIUS server certificate", "skipped", "already issued")
	} else if err := certs.EnsureServerCert(certs.ServerCertName); err != nil {
		res.fail("server_cert", "FreeRADIUS server certificate", err)
	} else {
		res.add("server_cert", "FreeRADIUS server certificate", "created", certs.ServerCertName+".crt issued by Tala WTE CA")
	}

	// 3. LDAP directory - RADIUS binds users against it for PEAP/MSCHAPv2.
	if ldap.HasUsers() {
		res.add("ldap_users", "LDAP user directory", "skipped", "directory already populated")
	} else {
		provRes, err := ldap.ProvisionDefault()
		if err != nil {
			res.fail("ldap_users", "LDAP user directory", err)
		} else {
			res.add("ldap_users", "LDAP user directory", "created",
				fmt.Sprintf("%d users provisioned from %s baseline", len(provRes.Users), provRes.CompanyName))
			res.Users = provRes.Users
		}
	}

	// 4. clients.conf - secret syncing already lives in ensureRADIUSClientsConf.
	secret := loadRADIUSSecret()
	if err := ensureRADIUSClientsConf(secret); err != nil {
		res.fail("radius_clients_conf", "FreeRADIUS clients.conf", err)
	} else {
		res.add("radius_clients_conf", "FreeRADIUS clients.conf", "created", "synced with shared secret")
	}

	// 5. EAP module - required for PEAP/TLS.
	if err := ensureRADIUSModule("eap"); err != nil {
		res.fail("radius_eap_module", "FreeRADIUS EAP module", err)
	} else {
		res.add("radius_eap_module", "FreeRADIUS EAP module", "created", "symlinked into mods-enabled")
	}

	// 6. LDAP module - wire mods-enabled/ldap to our slapd at 127.0.0.1:3389; rlm_ldap.so ships in freeradius-ldap.
	if err := ensureRADIUSLDAPPackage(); err != nil {
		res.fail("radius_ldap_package", "FreeRADIUS LDAP module package", err)
	} else if err := writeFreeRADIUSLDAPModule(); err != nil {
		res.fail("radius_ldap_module", "FreeRADIUS LDAP module", err)
	} else {
		res.add("radius_ldap_module", "FreeRADIUS LDAP module", "created",
			"wired to ldap://127.0.0.1:3389/dc=tala,dc=wte with admin bind")
	}

	// 6b. inner-tunnel virtual server - Debian's default already invokes ldap in authorize; just verify it exists.
	if _, err := os.Stat(freeradiusInnerTunnel); err != nil {
		res.fail("radius_inner_tunnel", "FreeRADIUS inner-tunnel site", err)
	} else {
		res.add("radius_inner_tunnel", "FreeRADIUS inner-tunnel site", "skipped", "Debian default already invokes ldap in authorize")
	}

	// 6a. Install our CA + server cert into FreeRADIUS's certs dir; the Debian EAP module's snakeoil paths
	// don't exist, so the config check fails and freeradius won't start until ours are wired in.
	if err := installRADIUSCerts(); err != nil {
		res.fail("radius_certs_installed", "FreeRADIUS certs installed", err)
	} else {
		res.add("radius_certs_installed", "FreeRADIUS certs installed", "created",
			fmt.Sprintf("copied ca.crt + %s.{crt,key} to %s", certs.ServerCertName, radiusCertDir))
	}

	if err := patchEAPModuleCerts(); err != nil {
		res.fail("radius_eap_cert_paths", "FreeRADIUS EAP cert paths", err)
	} else {
		res.add("radius_eap_cert_paths", "FreeRADIUS EAP cert paths", "created",
			"rewrote private_key_file/certificate_file/ca_file in mods-enabled/eap")
	}

	// 6c. slapd must be accepting connections before FreeRADIUS binds rlm_ldap.
	if ldap.IsRunning() {
		res.add("ldap_running", "OpenLDAP (slapd)", "skipped", "already accepting on 127.0.0.1:3389")
	} else if err := ldap.Start(); err != nil {
		res.fail("ldap_running", "OpenLDAP (slapd)", err)
	} else {
		res.add("ldap_running", "OpenLDAP (slapd)", "created", "slapd accepting on 127.0.0.1:3389")
	}

	// 7. Service. Validate the generated config, then restart so module + clients.conf changes take effect.
	if out, err := exec.Command("freeradius", "-C").CombinedOutput(); err != nil {
		res.fail("freeradius_config", "FreeRADIUS config check", fmt.Errorf("freeradius -C: %s", lastConfigError(out)))
	} else {
		res.add("freeradius_config", "FreeRADIUS config check", "created", "configuration valid")
	}
	if err := exec.Command("systemctl", "restart", "freeradius").Run(); err != nil {
		res.fail("freeradius_running", "FreeRADIUS service", err)
	} else {
		res.add("freeradius_running", "FreeRADIUS service", "created", "systemctl restart freeradius")
	}

	for _, s := range res.Steps {
		if s.Status == "failed" {
			res.OK = false
			break
		}
	}
	return res
}

func (r *EnterpriseProvisionResult) add(id, label, status, detail string) {
	log.Printf("[enterprise-provision] %-22s %-8s %s", id, status, detail)
	r.Steps = append(r.Steps, EnterpriseProvisionStep{ID: id, Label: label, Status: status, Detail: detail})
}

func (r *EnterpriseProvisionResult) fail(id, label string, err error) {
	r.add(id, label, "failed", err.Error())
}

// installRADIUSCerts copies the CA + RADIUS server cert/key into FreeRADIUS's certs dir and chowns them to freerad.
func installRADIUSCerts() error {
	if err := os.MkdirAll(radiusCertDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", radiusCertDir, err)
	}
	pairs := [][2]string{
		{filepath.Join(certs.CADir(), "ca.crt"), filepath.Join(radiusCertDir, "ca.pem")},
		{filepath.Join(certs.CADir(), certs.ServerCertName+".crt"), filepath.Join(radiusCertDir, "server.pem")},
		{filepath.Join(certs.CADir(), certs.ServerCertName+".key"), filepath.Join(radiusCertDir, "server.key")},
	}
	for _, p := range pairs {
		data, err := os.ReadFile(p[0])
		if err != nil {
			return fmt.Errorf("read %s: %w", p[0], err)
		}
		mode := os.FileMode(0o644)
		if filepath.Ext(p[1]) == ".key" {
			mode = 0o640
		}
		if err := os.WriteFile(p[1], data, mode); err != nil {
			return fmt.Errorf("write %s: %w", p[1], err)
		}
	}
	// Best-effort chown so the daemon can read after dropping privileges; the user name varies by distro.
	_ = exec.Command("chown", "-R", "freerad:freerad", radiusCertDir).Run()
	return nil
}

// EAP module cert path matchers for rewriting mods-enabled/eap; tolerant of whitespace so the patch is repeatable.
var (
	eapPrivateKeyRe  = regexp.MustCompile(`(?m)^([ \t]*)private_key_file\s*=.*$`)
	eapCertFileRe    = regexp.MustCompile(`(?m)^([ \t]*)certificate_file\s*=.*$`)
	eapCAFileRe      = regexp.MustCompile(`(?m)^([ \t]*)ca_file\s*=.*$`)
	eapPrivateKeyPwd = regexp.MustCompile(`(?m)^([ \t]*)private_key_password\s*=.*$`)
)

// patchEAPModuleCerts rewrites the tls-common block in mods-enabled/eap to use our CA and server cert. Idempotent.
func patchEAPModuleCerts() error {
	data, err := os.ReadFile(freeradiusEAPModule)
	if err != nil {
		return fmt.Errorf("read eap module: %w", err)
	}
	src := string(data)
	src = eapPrivateKeyRe.ReplaceAllString(src, fmt.Sprintf(`${1}private_key_file = %s/server.key`, radiusCertDir))
	src = eapCertFileRe.ReplaceAllString(src, fmt.Sprintf(`${1}certificate_file = %s/server.pem`, radiusCertDir))
	src = eapCAFileRe.ReplaceAllString(src, fmt.Sprintf(`${1}ca_file = %s/ca.pem`, radiusCertDir))
	// Our generated key has no passphrase; clear the default so freeradius loads it.
	src = eapPrivateKeyPwd.ReplaceAllString(src, `${1}private_key_password = ""`)
	if string(data) == src {
		return nil
	}
	return os.WriteFile(freeradiusEAPModule, []byte(src), 0o640)
}

// writeFreeRADIUSLDAPModule writes mods-enabled/ldap wired to our slapd at 127.0.0.1:3389, binding as
// cn=admin and mapping userPassword to control:Password-With-Header for inner-tunnel PAP/MSCHAPv2. Idempotent.
func writeFreeRADIUSLDAPModule() error {
	desired := fmt.Sprintf(`# Tala WTE - managed FreeRADIUS LDAP module config
# Points at the embedded OpenLDAP (slapd) instance on 127.0.0.1:3389.

ldap {
	server   = 'ldap://127.0.0.1:3389/'
	identity = 'cn=admin,dc=tala,dc=wte'
	password = '%s'
	base_dn  = 'dc=tala,dc=wte'

	sasl {
	}

	update {
		# Map slapd's userPassword into control:Password-With-Header.
		# FreeRADIUS auto-detects {SSHA}/{CRYPT} prefixes; plaintext values
		# end up as control:Cleartext-Password, which is what mschap and
		# pap need for inner-tunnel auth.
		control:Password-With-Header	+= 'userPassword'
		control:				+= 'radiusControlAttribute'
		request:				+= 'radiusRequestAttribute'
		reply:					+= 'radiusReplyAttribute'
	}

	user {
		base_dn = "ou=Users,${..base_dn}"
		filter  = "(uid=%%{%%{Stripped-User-Name}:-%%{User-Name}})"
		scope   = 'sub'
	}

	group {
		base_dn              = "ou=Groups,${..base_dn}"
		filter               = '(objectClass=groupOfNames)'
		scope                = 'sub'
		name_attribute       = cn
		membership_filter    = "(|(member=%%{control:Ldap-UserDn})(memberUid=%%{%%{Stripped-User-Name}:-%%{User-Name}}))"
		membership_attribute = 'member'
		cacheable_name       = 'no'
		cacheable_dn         = 'no'
	}

	options {
		chase_referrals = yes
		rebind          = yes
		res_timeout     = 10
		srv_timelimit   = 3
		net_timeout     = 1
		idle            = 60
		probes          = 3
		interval        = 3
	}

	tls {
		start_tls = no
	}

	pool {
		start             = 0
		min               = 0
		max               = ${thread[pool].max_servers}
		spare             = ${thread[pool].max_spare_servers}
		uses              = 0
		retry_delay       = 30
		lifetime          = 0
		idle_timeout      = 60
		connect_timeout   = 3.0
	}
}
`, ldap.AdminPassword())

	if existing, err := os.ReadFile(freeradiusLDAPModule); err == nil && string(existing) == desired {
		return nil
	}
	// Replace any existing symlink to mods-available/ldap with our file.
	_ = os.Remove(freeradiusLDAPModule)
	if err := os.WriteFile(freeradiusLDAPModule, []byte(desired), 0o640); err != nil {
		return fmt.Errorf("write %s: %w", freeradiusLDAPModule, err)
	}
	_ = exec.Command("chown", "root:freerad", freeradiusLDAPModule).Run()
	return nil
}

// ensureRADIUSLDAPPackage installs freeradius-ldap if rlm_ldap.so is missing (a separate Debian package).
func ensureRADIUSLDAPPackage() error {
	candidates := []string{
		"/usr/lib/freeradius/rlm_ldap.so",
		"/usr/lib/aarch64-linux-gnu/freeradius/rlm_ldap.so",
		"/usr/lib/x86_64-linux-gnu/freeradius/rlm_ldap.so",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return nil
		}
	}
	cmd := exec.Command("apt-get", "install", "-y", "freeradius-ldap")
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("apt-get install freeradius-ldap: %s: %w", out, err)
	}
	return nil
}

// ensureRADIUSModule symlinks mods-available/<name> into mods-enabled if not already present. Idempotent.
func ensureRADIUSModule(name string) error {
	src := "/etc/freeradius/3.0/mods-available/" + name
	dst := "/etc/freeradius/3.0/mods-enabled/" + name
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("%s not found - is freeradius package installed?", src)
	}
	if err := os.Symlink(src, dst); err != nil {
		return fmt.Errorf("symlink %s -> %s: %w", src, dst, err)
	}
	return nil
}
