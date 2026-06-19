// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package main

import (
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase"

	"github.com/vtemlabs/tala-wte/internal/client"
	"github.com/vtemlabs/tala-wte/internal/sim"
)

// bootSettleDelay gives the system (wireless firmware, namespaces, deps) time to
// come up before we try to restore networks or reconnect.
const bootSettleDelay = 8 * time.Second

// bootAutoStart restores running state after a reboot or crash so the box comes
// back the way the operator left it: a client reconnects to its last network and
// an AP restarts the networks that were running.
func bootAutoStart(app *pocketbase.PocketBase) {
	time.Sleep(bootSettleDelay)
	if clientMode() {
		cfg, ok := loadAutoconnect()
		if !ok {
			return
		}
		log.Printf("[boot] auto-reconnecting client to %q", cfg.SSID)
		if err := client.Get().Connect(cfg); err != nil {
			log.Printf("[boot] auto-reconnect to %q failed: %v", cfg.SSID, err)
		}
		return
	}
	autoStartNetworks(app)
}

// autoStartNetworks restarts every network whose persisted status is "running" so
// a server reboot brings its access points back automatically. It drives the
// existing start handler through an internal request, so the full start path
// (preflight, adapter allocation, hostapd) is reused unchanged; a network that
// cannot come back (e.g. its adapter is gone) is marked "error" for the operator.
// pendingAutostart holds the IDs of networks that were running before this boot,
// captured by resetNetworkStatuses before it clears their live status (the boot
// reconcile would otherwise erase the "was running" intent).
var pendingAutostart []string

func autoStartNetworks(app *pocketbase.PocketBase) {
	if len(pendingAutostart) == 0 {
		return
	}
	start := sim.StartHandler(app)
	for _, id := range pendingAutostart {
		rec, err := app.FindRecordById("networks", id)
		if err != nil {
			continue
		}
		ssid := rec.GetString("ssid")
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"auto_provision":true}`))
		req.SetPathValue("id", id)
		rr := httptest.NewRecorder()
		start(rr, req)
		if rr.Code == http.StatusOK {
			log.Printf("[boot] restarted network %q", ssid)
			continue
		}
		log.Printf("[boot] could not restart %q (HTTP %d): %s", ssid, rr.Code, strings.TrimSpace(rr.Body.String()))
		rec.Set("status", "error")
		if err := app.Save(rec); err != nil {
			log.Printf("[boot] failed to mark %q errored: %v", ssid, err)
		}
	}
	pendingAutostart = nil
}
