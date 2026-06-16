// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

package sim

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/vtemlabs/tala-wte/internal/certs"
	"github.com/vtemlabs/tala-wte/internal/ldap"
)

// EnterprisePreflight reports whether every dependency required to serve a WPA-Enterprise SSID is in place.
type EnterprisePreflight struct {
	OK     bool              `json:"ok"`
	Checks []EnterpriseCheck `json:"checks"`
}

// EnterpriseCheck is one item on the enterprise readiness checklist.
type EnterpriseCheck struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	OK          bool   `json:"ok"`
	Detail      string `json:"detail,omitempty"`
	AutoFixable bool   `json:"auto_fixable"`
}

const (
	freeradiusEAPModule   = "/etc/freeradius/3.0/mods-enabled/eap"
	freeradiusLDAPModule  = "/etc/freeradius/3.0/mods-enabled/ldap"
	freeradiusInnerTunnel = "/etc/freeradius/3.0/sites-enabled/inner-tunnel"
	freeradiusClientsConf = radiusClientsConf
)

// CheckEnterprisePreflight inspects the live system and returns the readiness checklist. It performs no mutations.
func CheckEnterprisePreflight() EnterprisePreflight {
	checks := []EnterpriseCheck{
		checkLDAPRunning(),
		checkLDAPUsers(),
		checkCA(),
		checkServerCert(),
		checkRADIUSClientsConf(),
		checkRADIUSEAPModule(),
		checkRADIUSLDAPModule(),
		checkFreeRADIUSRunning(),
	}
	ok := true
	for _, c := range checks {
		if !c.OK {
			ok = false
			break
		}
	}
	return EnterprisePreflight{OK: ok, Checks: checks}
}

func checkLDAPRunning() EnterpriseCheck {
	c := EnterpriseCheck{ID: "ldap_running", Label: "OpenLDAP (slapd) accepting connections", AutoFixable: true}
	if ldap.IsRunning() {
		c.OK = true
	} else {
		c.Detail = "slapd is not bound to ldap://127.0.0.1:3389"
	}
	return c
}

func checkLDAPUsers() EnterpriseCheck {
	c := EnterpriseCheck{ID: "ldap_users", Label: "LDAP directory has at least one user", AutoFixable: true}
	n, err := ldap.UserCount()
	switch {
	case err != nil:
		c.Detail = "could not query directory: " + err.Error()
	case n == 0:
		c.Detail = "ou=Users is empty - auto-fix will provision the default ACME Corp directory"
	default:
		c.OK = true
	}
	return c
}

func checkCA() EnterpriseCheck {
	c := EnterpriseCheck{ID: "ca", Label: "Certificate Authority initialized", AutoFixable: true}
	if certs.CAExists() {
		c.OK = true
	} else {
		c.Detail = "no CA found at " + filepath.Join(certs.CADir(), "ca.crt")
	}
	return c
}

func checkServerCert() EnterpriseCheck {
	c := EnterpriseCheck{ID: "server_cert", Label: "FreeRADIUS server certificate issued", AutoFixable: true}
	if certs.CertExists(certs.ServerCertName) {
		c.OK = true
	} else {
		c.Detail = "no " + certs.ServerCertName + ".crt under " + certs.CADir()
	}
	return c
}

func checkRADIUSClientsConf() EnterpriseCheck {
	c := EnterpriseCheck{ID: "radius_clients_conf", Label: "FreeRADIUS clients.conf in sync with shared secret", AutoFixable: true}
	if _, err := os.Stat(freeradiusClientsConf); err != nil {
		c.Detail = freeradiusClientsConf + " missing"
		return c
	}
	c.OK = true
	return c
}

func checkRADIUSEAPModule() EnterpriseCheck {
	c := EnterpriseCheck{ID: "radius_eap_module", Label: "FreeRADIUS EAP module enabled", AutoFixable: true}
	if _, err := os.Stat(freeradiusEAPModule); err == nil {
		c.OK = true
	} else {
		c.Detail = freeradiusEAPModule + " is not a symlink to mods-available/eap"
	}
	return c
}

func checkRADIUSLDAPModule() EnterpriseCheck {
	c := EnterpriseCheck{ID: "radius_ldap_module", Label: "FreeRADIUS LDAP module wired to embedded slapd", AutoFixable: true}
	data, err := os.ReadFile(freeradiusLDAPModule)
	if err != nil {
		c.Detail = freeradiusLDAPModule + " missing"
		return c
	}
	// Distinguish our managed config from Debian's default, which would silently fail to authenticate.
	content := string(data)
	switch {
	case !strings.Contains(content, "Tala WTE - managed"):
		c.Detail = "default Debian template still in place - won't bind to embedded slapd"
	case !strings.Contains(content, ":3389"):
		c.Detail = "configured server is not 127.0.0.1:3389"
	default:
		c.OK = true
	}
	return c
}

func checkFreeRADIUSRunning() EnterpriseCheck {
	c := EnterpriseCheck{ID: "freeradius_running", Label: "FreeRADIUS service is active", AutoFixable: true}
	if err := exec.Command("systemctl", "is-active", "--quiet", "freeradius").Run(); err == nil {
		c.OK = true
	} else {
		c.Detail = "systemctl is-active freeradius reports inactive"
	}
	return c
}
