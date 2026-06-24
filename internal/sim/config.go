// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package sim

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"math/big"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/pocketbase/pocketbase/core"

	"github.com/vtemlabs/tala-wte/internal/iface"
	"github.com/vtemlabs/tala-wte/internal/sim/pixie"
	"github.com/vtemlabs/tala-wte/pkg/hostapd"
)

const radiusClientsConf = "/etc/freeradius/3.0/clients.conf"

// radiusConfMu serializes clients.conf writes + freeradius reloads across concurrent enterprise network starts.
var radiusConfMu sync.Mutex

// regulatoryCountry returns the regulatory domain hostapd advertises (default US; the world domain "00" can't
// beacon 5 GHz). Override with TALA_COUNTRY_CODE.
func regulatoryCountry() string {
	if c := strings.TrimSpace(os.Getenv("TALA_COUNTRY_CODE")); c != "" {
		return strings.ToUpper(c)
	}
	return "US"
}

// fitWEPKey coerces input into a valid WEP key length hostapd accepts: hex stays hex, 5/13-char ASCII stays,
// anything else is fitted to a 13-char ASCII key (truncated or repeat-padded).
func fitWEPKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if (len(key) == 10 || len(key) == 26) && isHexKey(key) {
		return strings.ToLower(key)
	}
	r := []rune(key)
	if len(r) == 5 || len(r) == 13 {
		return key
	}
	if len(r) > 13 {
		return string(r[:13])
	}
	for len(r) < 13 {
		r = append(r, []rune(key)...)
	}
	return string(r[:13])
}

// formatWEPKey renders a wep_key0 value: a 10/26-hex key bare, anything else quoted as an ASCII key.
func formatWEPKey(key string) string {
	key = fitWEPKey(key)
	if (len(key) == 10 || len(key) == 26) && isHexKey(key) {
		return key
	}
	return `"` + key + `"`
}

func isHexKey(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// incMAC returns the MAC with its last byte incremented, used to derive the
// companion OWE-transition BSS BSSID from the radio's primary MAC.
func incMAC(mac string) string {
	hw, err := net.ParseMAC(mac)
	if err != nil || len(hw) != 6 {
		return mac
	}
	hw[5]++
	return hw.String()
}

// generateSecret creates a cryptographically random alphanumeric string.
func generateSecret(length int) (string, error) {
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

// sanitizeConfValue removes control characters so a value can't break out of its hostapd config line.
func sanitizeConfValue(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

// deriveSubnet returns the gateway and DHCP pool for an AP/LAN subnet. Only IPv4
// /24 is supported: gateway = .1, pool = .10 - .250. An empty or invalid CIDR
// falls back to the historical 10.0.0.0/24.
func deriveSubnet(cidr string) (gateway, dhcpStart, dhcpEnd string) {
	base := "10.0.0"
	if cidr != "" {
		if ip, _, err := net.ParseCIDR(strings.TrimSpace(cidr)); err == nil {
			if v4 := ip.To4(); v4 != nil {
				base = fmt.Sprintf("%d.%d.%d", v4[0], v4[1], v4[2])
			}
		}
	}
	return base + ".1", base + ".10", base + ".250"
}

// defaultChannelForBand returns a sensible default channel for a band, used when a
// confirmed adapter swap changes the network's band and the old channel no longer fits.
func defaultChannelForBand(band string) int {
	switch band {
	case "5":
		return 36
	case "6":
		return 1
	default:
		return 6
	}
}

// bandLabel maps a stored band ("2.4"/"5"/"6") to the adapter band label ("2.4 GHz" ...).
func bandLabel(band string) string {
	switch band {
	case "5":
		return "5 GHz"
	case "6":
		return "6 GHz"
	default:
		return "2.4 GHz"
	}
}

// shortBand maps an adapter band label back to the stored short form.
func shortBand(label string) string {
	switch {
	case strings.HasPrefix(label, "5"):
		return "5"
	case strings.HasPrefix(label, "6"):
		return "6"
	default:
		return "2.4"
	}
}

// bestBandForAdapter returns the band a substitute adapter should host. If the
// adapter can host the wanted band as an AP it returns "" (keep the configured
// band); otherwise it returns the most compatible band it does support, preferring
// 2.4 GHz. Returns "" when capability is unknown, so the configured band is kept.
func bestBandForAdapter(cand *iface.Adapter, band string) string {
	if band == "" {
		band = "2.4"
	}
	apBands := cand.APBands
	if len(apBands) == 0 {
		apBands = cand.Bands
	}
	if len(apBands) == 0 {
		return ""
	}
	wanted := bandLabel(band)
	for _, b := range apBands {
		if b == wanted {
			return ""
		}
	}
	for _, b := range apBands {
		if b == "2.4 GHz" {
			return "2.4"
		}
	}
	return shortBand(apBands[0])
}

// buildConfig constructs the hostapd.Config from a network record. ifName is the resolved adapter (the allocation
// guard may substitute a free one). nsGatewayIP is the host-side veth address used as the enterprise RADIUS server;
// pass "" for non-enterprise protocols.
func buildConfig(record *core.Record, ifName, ifMAC, nsGatewayIP string) *hostapd.Config {
	protocol := record.GetString("protocol")
	band := record.GetString("band")
	channel := record.GetInt("channel")
	if channel == 0 {
		channel = 6
	}

	// Strip control characters so a crafted value can't inject hostapd directives; SSID capped to 32 bytes.
	ssid := sanitizeConfValue(record.GetString("ssid"))
	if len(ssid) > 32 {
		ssid = ssid[:32]
	}
	passphrase := sanitizeConfValue(record.GetString("passphrase"))

	cfg := &hostapd.Config{
		Interface:   sanitizeConfValue(ifName),
		SSID:        ssid,
		Channel:     channel,
		APIsolate:   record.GetBool("client_isolation"),
		Hidden:      record.GetBool("hidden"),
		CountryCode: regulatoryCountry(),
	}

	switch band {
	case "5":
		cfg.HWMode = "a"
		cfg.IEEE80211N = true
		cfg.IEEE80211AC = true
	case "6":
		cfg.HWMode = "a"
		cfg.IEEE80211AX = true
		cfg.OpClass = 131
	default: // 2.4 GHz
		cfg.HWMode = "g"
		cfg.IEEE80211N = true
	}

	switch protocol {
	case "open":
		cfg.Protocol = hostapd.ProtocolOpen
	case "wep":
		cfg.Protocol = hostapd.ProtocolWEP
		// Use the sanitized passphrase (control chars already stripped) so a crafted
		// WEP key cannot inject a hostapd directive; formatWEPKey does not sanitize.
		cfg.WEPKey = formatWEPKey(passphrase)
		// WEP is incompatible with HT/VHT/HE; strip the band-derived radio modes.
		cfg.IEEE80211N = false
		cfg.IEEE80211AC = false
		cfg.IEEE80211AX = false
	case "wpa":
		cfg.Protocol = hostapd.ProtocolWPA
		cfg.Passphrase = passphrase
	case "wpa2":
		cfg.Protocol = hostapd.ProtocolWPA2
		cfg.Passphrase = passphrase
		// Optional PMKID exposure: stock hostapd omits the RSN PMKID KDE from
		// EAPOL msg 1/4 for WPA2-PSK; the embedded patched hostapd includes it,
		// enabling a clientless PMKID capture (hcxdumptool -> hashcat 22000).
		if record.GetBool("pmkid_exposed") {
			if bin, err := pixie.HostapdPath(); err != nil {
				log.Printf("[sim] WPA2 network %q: PMKID exposure requested but unavailable: %v (falling back to system hostapd)", ssid, err)
			} else {
				cfg.Binary = bin
				log.Printf("[sim] WPA2 network %q: PMKID exposed, using embedded hostapd %s (RSN PMKID in M1, clientless-capturable)", ssid, bin)
			}
		}
	case "wps":
		cfg.Protocol = hostapd.ProtocolWPS
		cfg.Passphrase = passphrase
		// A configured AP registrar PIN is what makes WPS attackable: without it
		// hostapd refuses the external-registrar exchange with EAP-Failure, so
		// reaver/bully have no PIN to recover. Derive a stable, valid 8-digit PIN
		// per network so the WPS PIN brute-force lab actually has a target.
		cfg.WPSPin = wpsPIN(record.Id)
		log.Printf("[sim] WPS network %q: ap_pin=%s (training target, recoverable via reaver/bully)", ssid, cfg.WPSPin)
		// Optional downgrade: swap in the embedded Pixie-Dust-vulnerable hostapd
		// (WPS secret nonces zeroed) so pixiewps can recover the PIN offline.
		// Off by default, every WPS network is online-PIN-only (Pixie-resistant),
		// which is how a modern AP behaves.
		if record.GetBool("wps_pixie") {
			if bin, err := pixie.HostapdPath(); err != nil {
				log.Printf("[sim] WPS network %q: Pixie downgrade requested but unavailable: %v (falling back to system hostapd)", ssid, err)
			} else {
				cfg.Binary = bin
				log.Printf("[sim] WPS network %q: Pixie-Dust downgrade ON, using embedded hostapd %s (recoverable via pixiewps)", ssid, bin)
			}
		}
	case "wpa3":
		cfg.Protocol = hostapd.ProtocolWPA3
		cfg.Passphrase = passphrase
		cfg.PMFMode = hostapd.PMFRequired
	case "wpa3_transition":
		cfg.Protocol = hostapd.ProtocolWPA3Transition
		cfg.Passphrase = passphrase
		cfg.PMFMode = hostapd.PMFOptional
	case "wpa2_enterprise":
		cfg.Protocol = hostapd.ProtocolWPA2Enterprise
		cfg.PMFMode = hostapd.PMFOptional
		applyEnterpriseConfig(cfg, record, nsGatewayIP)
	case "wpa3_enterprise":
		cfg.Protocol = hostapd.ProtocolWPA3Enterprise
		cfg.PMFMode = hostapd.PMFRequired
		applyEnterpriseConfig(cfg, record, nsGatewayIP)
	case "owe":
		// OWE / Enhanced Open: no passphrase, mandatory PMF.
		cfg.Protocol = hostapd.ProtocolOWE
		cfg.PMFMode = hostapd.PMFRequired
	case "wpa2_ft":
		// WPA2-PSK with 802.11r Fast Transition.
		cfg.Protocol = hostapd.ProtocolWPA2FT
		cfg.Passphrase = passphrase
		cfg.PMFMode = hostapd.PMFOptional
	case "owe_transition":
		// OWE transition: open primary BSS + hidden companion OWE BSS (downgrade target).
		// Needs a radio that can host two AP vifs; the bssids derive from the radio MAC.
		cfg.Protocol = hostapd.ProtocolOWETransition
		cfg.PMFMode = hostapd.PMFRequired
		cfg.RadioMAC = ifMAC
		cfg.OWESecondaryBSSID = incMAC(ifMAC)
		oweHidden := ssid + "-enc"
		if len(oweHidden) > 32 {
			oweHidden = oweHidden[:32]
		}
		cfg.OWEHiddenSSID = oweHidden
	}

	return cfg
}

// wpsPIN derives a stable, valid 8-digit WPS device PIN from a seed. The first 7
// digits come from a hash of the seed; the 8th is the WPS checksum digit. hostapd
// rejects an ap_pin without a valid checksum, and without a registrar PIN at all
// it refuses the WPS exchange (EAP-Failure), so this is what gives the WPS PIN
// brute-force lab an actual, recoverable target.
func wpsPIN(seed string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(seed))
	base := int(h.Sum32() % 10000000) // 7-digit body
	return fmt.Sprintf("%07d%d", base, wpsChecksum(base))
}

// wpsChecksum returns the WPS PIN checksum digit for a 7-digit PIN body, per the
// Wi-Fi Simple Config spec.
func wpsChecksum(pin int) int {
	accum := 0
	for pin > 0 {
		accum += 3 * (pin % 10)
		pin /= 10
		accum += pin % 10
		pin /= 10
	}
	return (10 - accum%10) % 10
}

func applyEnterpriseConfig(cfg *hostapd.Config, record *core.Record, nsGatewayIP string) {
	// hostapd runs inside the namespace; point RADIUS at the host-side veth IP (its gateway) so packets reach
	// FreeRADIUS on the host. clients.conf whitelists 192.168.0.0/16 so any concurrent enterprise namespace works.
	cfg.RADIUSAddr = nsGatewayIP
	cfg.RADIUSPort = 1812
	cfg.RADIUSSecret = loadRADIUSSecret()

	// Allow override via the per-network config_json (e.g. an off-host RADIUS).
	extra := record.GetString("config_json")
	if extra != "" {
		var m map[string]any
		if err := json.Unmarshal([]byte(extra), &m); err == nil {
			if addr, ok := m["radius_addr"].(string); ok && addr != "" {
				cfg.RADIUSAddr = sanitizeConfValue(addr)
			}
		}
	}

	// Sync clients.conf with this secret before hostapd starts, else the first enterprise start fails EAP.
	if err := ensureRADIUSClientsConf(cfg.RADIUSSecret); err != nil {
		log.Printf("[radius] failed to sync clients.conf with current secret: %v", err)
	}
}

// sanitizeRADIUSSecret strips control characters so a secret can't break out of its clients.conf line.
func sanitizeRADIUSSecret(s string) string {
	s = strings.TrimSpace(s)
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

func loadRADIUSSecret() string {
	if secret := sanitizeRADIUSSecret(os.Getenv("TALA_RADIUS_SECRET")); secret != "" {
		return secret
	}

	const secretFile = "/var/lib/tala-wte/radius/.shared_secret"
	if data, err := os.ReadFile(secretFile); err == nil && len(data) > 0 {
		if secret := sanitizeRADIUSSecret(string(data)); secret != "" {
			return secret
		}
	}

	generated, err := generateSecret(32)
	if err != nil {
		log.Fatalf("[radius] failed to generate random shared secret: %v", err)
	}

	if err := os.MkdirAll("/var/lib/tala-wte/radius", 0o750); err == nil {
		if err := os.WriteFile(secretFile, []byte(generated), 0o600); err != nil {
			log.Printf("[radius] failed to persist shared secret to %s: %v", secretFile, err)
		}
	}
	log.Printf("[radius] shared secret generated and persisted to %s", secretFile)
	return generated
}

// EnsureRADIUSClientsConf writes clients.conf with the given secret (host loopback
// plus the per-namespace veth range) and reloads FreeRADIUS. Exported so the
// settings handler reuses this canonical writer instead of emitting a
// localhost-only file that would drop namespace authorization between enterprise starts.
func EnsureRADIUSClientsConf(secret string) error {
	return ensureRADIUSClientsConf(sanitizeRADIUSSecret(secret))
}

// ensureRADIUSClientsConf writes clients.conf with the given secret and reloads FreeRADIUS. Idempotent. The file
// authorizes the host loopback and the 192.168.0.0/16 range covering all per-namespace veth pairs via one CIDR entry.
func ensureRADIUSClientsConf(secret string) error {
	if secret == "" {
		return fmt.Errorf("empty secret")
	}

	radiusConfMu.Lock()
	defer radiusConfMu.Unlock()

	desired := fmt.Sprintf(`# Tala WTE - managed by internal/sim
client localhost {
	ipaddr = 127.0.0.1
	secret = %s
}

client wte_namespaces {
	ipaddr = 192.168.0.0/16
	secret = %s
}
`, secret, secret)
	if existing, err := os.ReadFile(radiusClientsConf); err == nil && string(existing) == desired {
		return nil
	}

	if err := os.WriteFile(radiusClientsConf, []byte(desired), 0o640); err != nil {
		return fmt.Errorf("write clients.conf: %w", err)
	}

	// reload preserves accounting state; fall back to restart if the unit doesn't support it.
	if err := exec.Command("systemctl", "reload", "freeradius").Run(); err != nil {
		if rerr := exec.Command("systemctl", "restart", "freeradius").Run(); rerr != nil {
			return fmt.Errorf("reload freeradius: %w (restart fallback: %w)", err, rerr)
		}
	}
	log.Printf("[radius] clients.conf synced with current shared secret and freeradius reloaded")
	return nil
}
