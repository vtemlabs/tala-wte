// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

// Package client implements Tala WTE client mode: it joins another Tala WTE
// access point from an exported config, handles any captive portal, and
// generates realistic traffic to simulate an active network client.
package client

// Config is the connection profile exported from an AP network and imported into
// a client. It carries everything the client needs to join and get past a portal.
type Config struct {
	SSID        string `json:"ssid"`
	Protocol    string `json:"protocol"` // open, wep, wpa, wpa2, wps, wpa3, wpa3_transition, wpa2_enterprise, wpa3_enterprise
	Passphrase  string `json:"passphrase,omitempty"`
	Band        string `json:"band,omitempty"`
	Channel     int    `json:"channel,omitempty"`
	Hidden      bool   `json:"hidden,omitempty"`
	Identity    string `json:"identity,omitempty"`     // 802.1X EAP identity
	EAPPassword string `json:"eap_password,omitempty"` // 802.1X EAP password

	Portal PortalConfig `json:"portal"`
}

// PortalConfig tells the client how to get past a captive portal. When the portal
// requires login, Username/Password are submitted; otherwise the client just
// accepts the splash page to be allowlisted.
type PortalConfig struct {
	Enabled  bool   `json:"enabled"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	// AuthType is the portal's auth type; Fields carries a valid credential for it
	// (canonical field names, e.g. {"last_name":"Smith","room_number":"227"}) so a
	// deployed member passes a typed portal instead of guessing.
	AuthType string            `json:"auth_type,omitempty"`
	Fields   map[string]string `json:"fields,omitempty"`
}

// TrafficOptions are the runtime toggles chosen in the client console: which
// traffic types to generate, whether to target the local LAN/internet, and any
// operator-supplied target lists and credentials.
type TrafficOptions struct {
	Web       bool `json:"web"`       // HTTP/HTTPS browsing
	DNS       bool `json:"dns"`       // DNS lookups
	Ping      bool `json:"ping"`      // ICMP + local LAN chatter
	Downloads bool `json:"downloads"` // file downloads / bandwidth
	Creds     bool `json:"creds"`     // credentialed logins (capturable on the wire)
	Domain    bool `json:"domain"`    // Windows/domain chatter: LLMNR+NBT-NS+mDNS lookups and SMB NTLM auth (responder/MITM bait)
	Local     bool `json:"local"`     // target the local LAN
	Internet  bool `json:"internet"`  // target internet hosts

	// Operator-supplied target lists; when present they replace the built-in
	// defaults for that generator, so training traffic hits known hosts.
	URLs    []string `json:"urls,omitempty"`    // URLs to browse
	Domains []string `json:"domains,omitempty"` // domains to resolve
	IPs     []string `json:"ips,omitempty"`     // IPs to ping/connect

	// Credentials drive login traffic. Sent in cleartext over HTTP (and HTTP Basic)
	// on purpose so trainees can capture and analyze them; a captive-portal/decrypt
	// lab is the whole point.
	Credentials []Credential `json:"credentials,omitempty"`
}

// Credential is a login the client replays as traffic: an HTTP Basic GET plus a
// form POST of username/password to the given URL.
type Credential struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// Status is the live state reported to the client console.
type Status struct {
	Mode        string `json:"mode"` // always "client"
	Connected   bool   `json:"connected"`
	SSID        string `json:"ssid"`
	Interface   string `json:"interface"`
	IP          string `json:"ip"`
	Gateway     string `json:"gateway"`
	PortalState string `json:"portal_state"` // none, detected, passed, failed
	Generating  bool   `json:"generating"`
	Requests    int64  `json:"requests"`
	BytesRx     int64  `json:"bytes_rx"`
	Errors      int64  `json:"errors"`
	Cycling     bool   `json:"cycling"` // reconnect cycling (handshake capture) is active
	Cycles      int    `json:"cycles"`  // completed reconnect cycles
	Arch        string `json:"arch"`    // host CPU arch (amd64/arm64) for leader-pushed updates
	Version     string `json:"version"` // member software version, shown on the leader
	// Wireless-adapter health. A client with no usable adapter cannot generate
	// wireless traffic, so it is not "ready" regardless of being reachable.
	Adapters            int      `json:"adapters"`                 // usable (driver-supported, non-virtual) wireless adapters
	AdaptersUnsupported int      `json:"adapters_unsupported"`     // adapters present but without a working driver
	AdapterNames        []string `json:"adapter_names,omitempty"`  // model + chipset of each usable adapter, shown on the leader
	AdapterLimits       []string `json:"adapter_limits,omitempty"` // capability limits of the member's adapters, shown on the leader
	RadioWedged         bool     `json:"radio_wedged,omitempty"`   // radio stopped answering nl80211 (driver wedge); needs a power-cycle/replug
	LastError           string   `json:"last_error,omitempty"`
	LastEvent           string   `json:"last_event,omitempty"`
}
