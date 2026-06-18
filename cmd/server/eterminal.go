// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package main

// Sets up the vendored e-terminal shell environment for the operator account.
// e-terminal is embedded in the binary, so this extracts the baked-in copy and
// runs its install.sh; idempotent and non-fatal. The installer runs AS ROOT
// (it needs apt + /usr/local) but with the operator's HOME/USER so configs land
// in their home, skipping the shell change, then chowns the result back.

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/vtemlabs/tala-wte/internal/eterminal"
)

// bootstrapETerminal ensures the operator account has e-terminal set up.
func bootstrapETerminal(operator string) {
	if os.Geteuid() != 0 {
		return // need root to install packages + write the operator's home
	}
	operator = strings.TrimSpace(operator)
	if operator == "" || operator == "root" {
		return // tala-wte targets a regular operator account, not root
	}
	u, err := user.Lookup(operator)
	if err != nil || u.HomeDir == "" {
		fmt.Printf("-> e-terminal: operator %q not found; skipping\n", operator)
		return
	}
	home := u.HomeDir
	root := filepath.Join(home, ".e-terminal")

	if !eterminal.HasInstaller() {
		fmt.Println("-> e-terminal not vendored in this build; skipping (run `make eterminal` before building)")
		return
	}

	// Detect setup via a marker install.sh creates, not the repo dir (uninstall leaves the dir behind).
	if fileExists(filepath.Join(home, ".local/bin/e-session-log")) ||
		fileExists(filepath.Join(home, ".config/e-terminal/commit")) {
		fmt.Printf("-> e-terminal already set up for %s\n", operator)
		return
	}

	fmt.Printf("-> installing e-terminal for %s (this pulls packages; may take a few minutes)\n", operator)

	// Reset the share dir: a prior partial install can leave a self-referential symlink that aborts the installer.
	_ = os.RemoveAll(filepath.Join(home, ".local/share/e-terminal"))

	// Extract the embedded copy only if the repo isn't already on disk.
	if !fileExists(filepath.Join(root, "install.sh")) {
		if err := eterminal.ExtractTo(root); err != nil {
			fmt.Printf("-> e-terminal extract failed: %v\n", err)
			return
		}
	}

	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", filepath.Join(root, "install.sh"))
	cmd.Dir = home
	// Run as root with the operator's HOME/USER. SKIP_SHELL_CHANGE (login shell managed by the deploy); SKIP_FONT/SKIP_GHOSTTY (headless web terminal).
	cmd.Env = append(
		os.Environ(),
		"HOME="+home,
		"USER="+operator,
		"LOGNAME="+operator,
		"DEBIAN_FRONTEND=noninteractive",
		"SKIP_FONT=1",
		"SKIP_GHOSTTY=1",
		"SKIP_SHELL_CHANGE=1",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("-> e-terminal install did not finish cleanly: %v\n", err)
		if len(out) > 0 {
			tail := out
			if len(tail) > 1500 {
				tail = tail[len(tail)-1500:]
			}
			fmt.Printf("   --- install.sh tail ---\n   %s\n", strings.ReplaceAll(string(tail), "\n", "\n   "))
		}
		fmt.Println("   re-run `sudo tala-wte install` once the box has network; e-terminal is non-fatal")
		return // leave the extracted tree so a re-run resumes from it
	}

	// install.sh wrote into the operator's home as root; hand it back to them.
	chownOperatorTree(home, uid, gid)
	fmt.Printf("-> e-terminal installed for %s (zsh + starship + session logging)\n", operator)
}

// chownOperatorTree fixes ownership of the paths e-terminal touched, since install.sh ran as root.
func chownOperatorTree(home string, uid, gid int) {
	for _, p := range []string{
		".e-terminal", ".zshrc", ".zshrc.local", ".zsh_history",
		".config", ".local", ".cache",
	} {
		full := filepath.Join(home, p)
		_ = filepath.WalkDir(full, func(path string, _ fs.DirEntry, err error) error {
			if err != nil {
				return nil //nolint:nilerr // skip missing entries; chowning the rest is best-effort
			}
			_ = os.Lchown(path, uid, gid)
			return nil
		})
	}
}

// wipeETerminalArtifacts kills the session recorder, shreds every ~/terminal_logs, and removes the recorder binaries. Used by uninstall --purge.
func wipeETerminalArtifacts() {
	for _, p := range []string{"e-session-rec", "e-session-log"} {
		_ = exec.Command("pkill", "-f", p).Run()
	}
	homes := []string{"/root"}
	if entries, err := os.ReadDir("/home"); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				homes = append(homes, filepath.Join("/home", e.Name()))
			}
		}
	}
	recorders := []string{"e-session-log", "e-session-rec", "e-session-view"}
	for _, h := range homes {
		logs := filepath.Join(h, "terminal_logs")
		if fileExists(logs) {
			// Best-effort shred (0600 log files) before removal.
			_ = exec.Command("sh", "-c", fmt.Sprintf("find %q -type f -exec shred -uz {} + 2>/dev/null", logs)).Run()
			_ = os.RemoveAll(logs)
		}
		for _, b := range recorders {
			_ = os.Remove(filepath.Join(h, ".local/bin", b))
		}
	}
	for _, b := range recorders {
		_ = os.Remove(filepath.Join("/usr/local/bin", b))
	}
}

// fileExists reports whether path exists (file or dir).
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
