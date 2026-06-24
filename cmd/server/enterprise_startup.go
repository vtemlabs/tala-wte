// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package main

import (
	"log"
	"time"

	"github.com/vtemlabs/tala-wte/internal/ldap"
	"github.com/vtemlabs/tala-wte/internal/sim"
)

// provisionEnterpriseOnStartup brings the WPA-Enterprise dependency stack (CA,
// RADIUS server certificate, embedded LDAP directory, and freeradius-ldap wiring)
// to a known-good state at boot. Without it a fresh install serves a 412 the first
// time anyone starts an enterprise SSID (until the operator finds the provision
// step), which reads as "enterprise is broken." Every provision step is idempotent
// and skips itself when its invariant already holds, so this is a no-op on an
// already-provisioned box and safe to run on every boot. Runs in its own
// goroutine; any failure is logged and still surfaces through the normal
// per-start preflight.
func provisionEnterpriseOnStartup() {
	if sim.CheckEnterprisePreflight().OK {
		return
	}
	// Wait briefly for the embedded slapd to accept connections so the LDAP
	// directory step has somewhere to write.
	for i := 0; i < 30 && !ldap.IsRunning(); i++ {
		time.Sleep(time.Second)
	}
	res := sim.AutoProvisionEnterprise()
	if res.OK {
		log.Printf("[enterprise] startup auto-provision complete; WPA-Enterprise targets are ready")
		return
	}
	var failed []string
	for _, s := range res.Steps {
		if s.Status == "failed" {
			failed = append(failed, s.ID)
		}
	}
	log.Printf("[enterprise] startup auto-provision incomplete (failed steps: %v); run it from Settings or POST /api/wte/enterprise/provision", failed)
}
