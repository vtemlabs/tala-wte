// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

package sim

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/pocketbase/pocketbase/core"

	"github.com/vtemlabs/tala-wte/internal/iface"
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

// buildSwapProposal describes a proposed adapter substitution for a network whose saved
// radio is gone, including whether the candidate can host the saved band as an AP and,
// if not, which band it will fall back to.
func buildSwapProposal(missing string, cand *iface.Adapter, band string) map[string]any {
	if band == "" {
		band = "2.4"
	}
	label := cand.Interface
	switch {
	case cand.Manufacturer != "" && cand.DeviceModel != "":
		label = fmt.Sprintf("%s %s", cand.Manufacturer, cand.DeviceModel)
	case cand.Driver != "":
		label = cand.Driver
	}

	apBands := cand.APBands
	if len(apBands) == 0 {
		apBands = cand.Bands
	}
	wanted := bandLabel(band)
	bandOK := false
	for _, b := range apBands {
		if b == wanted {
			bandOK = true
			break
		}
	}
	suggested := band
	reason := ""
	if !bandOK && len(apBands) > 0 {
		suggested = shortBand(apBands[0])
		for _, b := range apBands {
			if b == "2.4 GHz" { // prefer the most compatible band
				suggested = "2.4"
				break
			}
		}
		reason = fmt.Sprintf("%s cannot host a %s access point; the network will switch to %s", cand.Interface, wanted, bandLabel(suggested))
	}

	return map[string]any{
		"needs_adapter_choice": true,
		"error":                fmt.Sprintf("configured adapter %q is not connected", missing),
		"missing":              missing,
		"proposed":             map[string]any{"interface": cand.Interface, "label": label},
		"current_band":         band,
		"band_ok":              bandOK,
		"suggested_band":       suggested,
		"band_reason":          reason,
	}
}

// buildConfig constructs the hostapd.Config from a network record. ifName is the resolved adapter (the allocation
// guard may substitute a free one). nsGatewayIP is the host-side veth address used as the enterprise RADIUS server;
// pass "" for non-enterprise protocols.
func buildConfig(record *core.Record, ifName string, nsGatewayIP string) *hostapd.Config {
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
		cfg.WEPKey = formatWEPKey(record.GetString("passphrase"))
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
	case "wps":
		cfg.Protocol = hostapd.ProtocolWPS
		cfg.Passphrase = passphrase
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
	}

	return cfg
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
