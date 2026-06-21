// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. See the LICENSE file.

package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase"

	"github.com/vtemlabs/tala-wte/internal/api"
	"github.com/vtemlabs/tala-wte/internal/discovery"
	"github.com/vtemlabs/tala-wte/internal/version"
)

// startMDNSAdvertiser advertises this instance on the LAN over mDNS so a pack
// leader elsewhere can find it without knowing its address - which covers fresh
// installs and hosts whose DHCP lease changes. It advertises for the process
// lifetime.
func startMDNSAdvertiser() {
	role := "leader"
	if clientMode() {
		role = "member"
	}
	host, _ := os.Hostname()
	if host == "" {
		host = "tala-wte"
	}
	if _, err := discovery.Advertise(host, role, version.Version, 8443); err != nil {
		log.Printf("[mdns] advertise failed: %v", err)
		return
	}
	log.Printf("[mdns] advertising %s as %s on _tala-wte._tcp", host, role)
}

// packDiscoveredHandler browses the LAN over mDNS for other Tala WTE instances so
// the leader can find potential members (and other leaders) without addresses.
// This instance is filtered out of the results.
func packDiscoveredHandler(_ *pocketbase.PocketBase) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		peers, err := discovery.Browse(2500 * time.Millisecond)
		if err != nil {
			api.WriteErr(w, http.StatusInternalServerError, "discovery failed: "+err.Error())
			return
		}
		host, _ := os.Hostname()
		out := make([]discovery.Peer, 0, len(peers))
		for _, p := range peers {
			if strings.EqualFold(p.Name, host) {
				continue // skip self
			}
			out = append(out, p)
		}
		api.WriteJSON(w, map[string]any{"peers": out})
	}
}
