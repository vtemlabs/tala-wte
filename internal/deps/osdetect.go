// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package deps

// Distro detection so the dependency installer adapts across the apt-family
// targets (Debian, Ubuntu, Kali, Raspberry Pi OS, Mint, Pop!_OS).

import (
	"bufio"
	"log"
	"os"
	"os/exec"
	"strings"
)

// osInfo is the subset of /etc/os-release the installer branches on.
type osInfo struct {
	ID         string // "debian" | "ubuntu" | "kali" | "raspbian" | "linuxmint" | "pop" | ...
	IDLike     string // space-separated chain, e.g. "ubuntu debian" for derivatives
	Version    string // VERSION_ID, e.g. "13" or "24.04"
	Codename   string // VERSION_CODENAME, e.g. "trixie", "noble", "kali-rolling"
	PrettyName string // PRETTY_NAME, e.g. "Debian GNU/Linux 13 (trixie)"
}

// readOSRelease parses /etc/os-release. Returns a zero-value struct (empty ID)
// when the file is missing or unreadable.
func readOSRelease() osInfo {
	out := osInfo{}
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return out
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := line[:eq]
		val := strings.Trim(line[eq+1:], `"`)
		switch key {
		case "ID":
			out.ID = strings.ToLower(val)
		case "ID_LIKE":
			out.IDLike = strings.ToLower(val)
		case "VERSION_ID":
			out.Version = val
		case "VERSION_CODENAME":
			out.Codename = strings.ToLower(val)
		case "PRETTY_NAME":
			out.PrettyName = val
		}
	}
	return out
}

// isAptFamily reports whether the distro uses apt-get.
func (o osInfo) isAptFamily() bool {
	known := map[string]bool{
		"debian": true, "ubuntu": true, "kali": true,
		"raspbian": true, "linuxmint": true, "pop": true,
		"elementary": true, "zorin": true, "neon": true,
	}
	if known[o.ID] {
		return true
	}
	for _, id := range strings.Fields(o.IDLike) {
		if known[strings.ToLower(id)] {
			return true
		}
	}
	return false
}

// isUbuntuLike reports whether the distro follows the Ubuntu firmware/repo
// convention (firmware in universe) rather than Debian's.
func (o osInfo) isUbuntuLike() bool {
	switch o.ID {
	case "ubuntu", "linuxmint", "pop", "elementary", "zorin", "neon":
		return true
	}
	return strings.Contains(o.IDLike, "ubuntu")
}

// ensureUbuntuUniverse enables the universe repo so firmware and wireless
// tooling are installable on Ubuntu. Best-effort and idempotent.
func ensureUbuntuUniverse() {
	if _, err := exec.LookPath("add-apt-repository"); err != nil {
		log.Println("[deps] installing software-properties-common (needed to enable universe repo)...")
		if err := runSystemCmd("apt-get", "install", "-y", "software-properties-common"); err != nil {
			log.Printf("[deps] WARNING: could not install software-properties-common: %v", err)
			log.Println("[deps] proceeding without universe - some firmware/tooling installs may fail")
			return
		}
	}
	log.Println("[deps] ensuring universe repo is enabled (Ubuntu)...")
	if err := runSystemCmd("add-apt-repository", "-y", "universe"); err != nil {
		log.Printf("[deps] WARNING: add-apt-repository universe failed: %v", err)
	}
}
