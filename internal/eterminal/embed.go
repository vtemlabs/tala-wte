// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

// Package eterminal embeds the e-terminal shell setup into the binary so the
// installer can set it up without cloning anything at install time.
package eterminal

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// assets holds the vendored e-terminal tree plus a placeholder so the package
// compiles on a clean checkout. The `all:` prefix pulls in dotfiles.
//
//go:embed all:assets
var assets embed.FS

const vendorRoot = "assets/e-terminal"

// HasInstaller reports whether a vendored e-terminal (with its install.sh) is
// baked into this build.
func HasInstaller() bool {
	_, err := fs.Stat(assets, vendorRoot+"/install.sh")
	return err == nil
}

// ExtractTo writes the embedded e-terminal tree to dest. go:embed flattens
// every file to 0444, so scripts and recorder binaries are restored to 0755.
func ExtractTo(dest string) error {
	return fs.WalkDir(assets, vendorRoot, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(vendorRoot, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := assets.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(rel, ".sh") ||
			strings.HasPrefix(rel, "config/bin/") ||
			strings.HasPrefix(rel, "config/tmux/scripts/") {
			mode = 0o755
		}
		return os.WriteFile(target, data, mode)
	})
}
