// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

// Package pixie embeds the deliberately-weakened hostapd that Tala WTE uses for
// opt-in vulnerable lab targets, and extracts it on demand. It is hostapd 2.11
// with two changes: the WPS enrollee/registrar secret nonces (E-S1/E-S2) zeroed
// so pixiewps recovers the WPS PIN offline, and the RSN PMKID KDE emitted in
// EAPOL msg 1/4 for WPA2-PSK so the PMKID is clientlessly capturable. Stock
// hostapd does neither; this build is used ONLY for networks that opt in
// (wps_pixie or pmkid_exposed). Every other network uses system hostapd.
package pixie

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

//go:embed hostapd-amd64 hostapd-arm64
var binaries embed.FS

var (
	extractOnce sync.Once
	extractPath string
	extractErr  error
)

// HostapdPath writes the embedded patched hostapd for the running architecture
// to a stable temp path and returns it. Extraction happens once per process;
// later calls reuse the same file. Returns an error if no build is embedded for
// the current GOARCH.
func HostapdPath() (string, error) {
	extractOnce.Do(func() {
		name := "hostapd-" + runtime.GOARCH
		data, err := binaries.ReadFile(name)
		if err != nil {
			extractErr = fmt.Errorf("pixie hostapd not available for %s: %w", runtime.GOARCH, err)
			return
		}
		dst := filepath.Join(os.TempDir(), "tala-wte-hostapd-pixie")
		if err := os.WriteFile(dst, data, 0o700); err != nil {
			extractErr = fmt.Errorf("write pixie hostapd: %w", err)
			return
		}
		// WriteFile only sets perms on create; force exec bits in case a stale
		// copy from an earlier build was left non-executable.
		if err := os.Chmod(dst, 0o700); err != nil {
			extractErr = fmt.Errorf("chmod pixie hostapd: %w", err)
			return
		}
		extractPath = dst
	})
	return extractPath, extractErr
}
