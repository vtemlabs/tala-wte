// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

// Package pixie ships a Pixie-Dust-vulnerable hostapd build embedded in the
// binary and extracts it on demand. The build is hostapd 2.11 with the WPS
// enrollee/registrar secret nonces (E-S1/E-S2) forced to zero, so pixiewps can
// recover the WPS PIN offline in seconds. Stock hostapd uses a strong RNG for
// those nonces and cannot be Pixie'd; this build is used ONLY for WPS lab
// networks that explicitly opt into the downgrade. Every other network, and
// any WPS network without the downgrade, uses system hostapd unchanged.
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

// HostapdPath writes the embedded Pixie-vulnerable hostapd for the running
// architecture to a stable temp path and returns it. Extraction happens once
// per process; later calls reuse the same file. Returns an error if no build is
// embedded for the current GOARCH.
func HostapdPath() (string, error) {
	extractOnce.Do(func() {
		name := "hostapd-" + runtime.GOARCH
		data, err := binaries.ReadFile(name)
		if err != nil {
			extractErr = fmt.Errorf("pixie hostapd not available for %s: %w", runtime.GOARCH, err)
			return
		}
		dst := filepath.Join(os.TempDir(), "tala-wte-hostapd-pixie")
		if err := os.WriteFile(dst, data, 0o755); err != nil {
			extractErr = fmt.Errorf("write pixie hostapd: %w", err)
			return
		}
		// WriteFile only sets perms on create; force exec bits in case a stale
		// copy from an earlier build was left non-executable.
		if err := os.Chmod(dst, 0o755); err != nil {
			extractErr = fmt.Errorf("chmod pixie hostapd: %w", err)
			return
		}
		extractPath = dst
	})
	return extractPath, extractErr
}
