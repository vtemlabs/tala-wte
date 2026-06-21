// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package deps

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

var requiredPackages = []string{
	"hostapd",
	"dnsmasq",
	"slapd",
	"ldap-utils",
	"freeradius",
	"freeradius-ldap",
	"freeradius-utils",
	"tshark",
	"iptables",
	"iproute2",
	"curl",
	"pciutils", // PCI adapter discovery
	"usbutils", // USB adapter discovery
	"hwdata",   // usb.ids for adapter vendor/product naming fallback
	"rfkill",
	"iw",            // modern wireless tooling (wireless-tools/iwconfig is obsolete and dropped on newer distros)
	"wpasupplicant", // client-mode association (members join networks via wpa_supplicant)
	"tcpdump",       // BPF filter validation in the capture engine
	"psmisc",        // fuser, for releasing ports held before slapd starts
	"procps",        // pgrep/pkill, for service status checks and cleanup
	"git",
}

// criticalPackages are the ones Tala WTE cannot run without. The rest are
// best-effort: a package that is obsolete, renamed, or absent on a given distro
// is skipped rather than aborting the whole install, so the installer adapts to
// any apt-based system.
var criticalPackages = map[string]bool{
	"hostapd":  true,
	"dnsmasq":  true,
	"iw":       true,
	"iproute2": true,
	"iptables": true,
	"tshark":   true,
}

// optionalFirmware returns best-effort firmware packages for physical wireless
// adapters by distro family: Ubuntu and derivatives ship the monolithic
// linux-firmware, while Debian/Kali/Raspberry Pi OS split it into firmware-*
// packages that don't exist on Ubuntu.
func optionalFirmware(osr osInfo) []string {
	if osr.isUbuntuLike() {
		// Ubuntu ships one monolithic package covering every driver's firmware.
		return []string{"linux-firmware"}
	}
	// Debian/Kali split firmware per vendor. Install the full wireless set so any
	// adapter works with no manual step; installOptional skips any package with no
	// install candidate on this release, so the list adapts per OS version.
	return []string{
		"firmware-mediatek",        // MediaTek MT76xx/MT79xx (ALFA AWUS036AXML/ACM, AX-class USB)
		"firmware-realtek",         // Realtek RTL8xxx USB adapters
		"firmware-atheros",         // Atheros (Debian <= bookworm)
		"firmware-ath9k-htc",       // Atheros AR9271 (ALFA AWUS036NHA; Debian >= trixie)
		"firmware-iwlwifi",         // Intel wireless
		"firmware-libertas",        // Marvell Libertas / SD8xxx
		"firmware-brcm80211",       // Broadcom
		"firmware-ti-connectivity", // TI WiLink
		"firmware-zd1211",          // ZyDAS ZD1211
		"firmware-misc-nonfree",    // catch-all for other non-free driver firmware
	}
}

// ensureAptSources ensures the Debian non-free-firmware component is enabled.
func ensureAptSources() {
	b, err := os.ReadFile("/etc/apt/sources.list")
	if err != nil {
		return
	}
	// Only active (non-comment) lines count: a commented-out cdrom entry that
	// mentions non-free-firmware must not be mistaken for it being available.
	for _, line := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if strings.Contains(t, "non-free-firmware") {
			return
		}
	}
	log.Println("[deps] Configuring APT sources for non-free-firmware...")
	cmd := exec.Command("sh", "-c", `sed -i -E '/^[[:space:]]*#/!s/[[:space:]]main$/ main non-free-firmware non-free/' /etc/apt/sources.list`)
	_ = cmd.Run()
	_ = runSystemCmd("apt-get", "update")
}

// Intentionally empty: capture drivers load on device probe and are preloaded
// at boot via /etc/modules-load.d/tala-wte-capture.conf (see usbrecover.go).
var requiredModules = []struct {
	name string
	args []string
}{}

// functionalCapabilities maps each Tala WTE feature to the runtime binaries it
// needs. After install these are verified directly so a missing or renamed
// package on any distro surfaces immediately as a clear capability error rather
// than failing later at runtime. Checking binaries (not package names) keeps the
// gate correct across distros that name or split packages differently.
var functionalCapabilities = []struct {
	name     string
	bins     []string
	critical bool
}{
	{"access points (hostapd)", []string{"hostapd"}, true},
	{"DHCP and captive DNS (dnsmasq)", []string{"dnsmasq"}, true},
	{"wireless control (iw)", []string{"iw"}, true},
	{"client association (wpa_supplicant)", []string{"wpa_supplicant"}, true},
	{"networking and namespaces (iproute2)", []string{"ip"}, true},
	{"firewall and NAT (iptables)", []string{"iptables"}, true},
	{"packet capture and analysis (tshark)", []string{"tshark", "capinfos"}, true},
	{"directory services (OpenLDAP)", []string{"slapd", "slapadd", "ldapsearch", "ldapadd"}, true},
	{"RADIUS authentication (FreeRADIUS)", []string{"freeradius"}, true},
	{"BPF filter validation (tcpdump)", []string{"tcpdump"}, false},
	{"process management (procps/psmisc)", []string{"pgrep", "pkill", "fuser"}, false},
}

// verifyFunctionality confirms every runtime binary Tala WTE depends on is
// present. It fails if any critical capability is unavailable, so an install can
// never report success while a core feature is broken.
func verifyFunctionality() error {
	missingCritical := []string{}
	for _, cap := range functionalCapabilities {
		absent := []string{}
		for _, b := range cap.bins {
			if _, err := exec.LookPath(b); err != nil {
				absent = append(absent, b)
			}
		}
		switch {
		case len(absent) == 0:
			log.Printf("[deps] OK: %s", cap.name)
		case cap.critical:
			log.Printf("[deps] MISSING (critical): %s -> %v", cap.name, absent)
			missingCritical = append(missingCritical, absent...)
		default:
			log.Printf("[deps] DEGRADED: %s -> %v (feature limited, continuing)", cap.name, absent)
		}
	}
	if len(missingCritical) > 0 {
		return fmt.Errorf("core functionality unavailable, missing binaries: %v", missingCritical)
	}
	return nil
}

// InstallPackages installs an arbitrary apt package list resiliently: it ensures
// the right repos, skips packages with no install candidate on this distro, and
// falls back to per-package installs so one bad package never blocks the rest.
// Used by client-mode install; a no-op on non-apt systems.
func InstallPackages(pkgs []string) error {
	osr := readOSRelease()
	if osr.ID != "" && !osr.isAptFamily() {
		log.Printf("[deps] %s is not apt-family; install these manually: %v", osr.ID, pkgs)
		return nil
	}
	if aptPath, _ := exec.LookPath("apt-get"); aptPath == "" {
		log.Printf("[deps] apt-get not found; install these manually: %v", pkgs)
		return nil
	}
	if osr.isUbuntuLike() {
		ensureUbuntuUniverse()
	} else {
		ensureAptSources()
	}

	missing := []string{}
	for _, pkg := range pkgs {
		if !isInstalled(pkg) {
			missing = append(missing, pkg)
		}
	}
	if len(missing) == 0 {
		log.Println("[deps] All client packages are satisfied.")
		return nil
	}
	log.Printf("[deps] Missing client packages: %v", missing)
	if err := runSystemCmd("apt-get", "update"); err != nil {
		log.Printf("[deps] apt-get update failed: %v (continuing)", err)
	}
	installable := []string{}
	for _, pkg := range missing {
		if hasCandidate(pkg) {
			installable = append(installable, pkg)
		} else {
			log.Printf("[deps] No install candidate for %q on this system; skipping.", pkg)
		}
	}
	if len(installable) > 0 {
		if err := runSystemCmd("apt-get", append([]string{"install", "-y"}, installable...)...); err != nil {
			log.Printf("[deps] Batch install failed (%v); retrying individually.", err)
			for _, pkg := range installable {
				if err := runSystemCmd("apt-get", "install", "-y", pkg); err != nil {
					log.Printf("[deps] Failed to install %q: %v", pkg, err)
				}
			}
		}
	}
	return nil
}

// VerifyAndInstall checks for missing packages and installs them via apt-get,
// then ensures required kernel modules are loaded.
func VerifyAndInstall() error {
	log.Println("[deps] Verifying system dependencies...")

	osr := readOSRelease()
	if osr.PrettyName != "" {
		log.Printf("[deps] Detected OS: %s (id=%s codename=%s)", osr.PrettyName, osr.ID, osr.Codename)
	}

	// Auto-install is apt-only. On other distros the operator manages packages
	// with their own package manager; the functionality check below still runs
	// so a missing core tool is reported rather than failing later at runtime.
	apt := true
	if osr.ID != "" && !osr.isAptFamily() {
		log.Printf("[deps] %s is not an apt-family distro; skipping automatic package installation. Install the required tools with your package manager.", osr.ID)
		apt = false
	} else if aptPath, _ := exec.LookPath("apt-get"); aptPath == "" {
		log.Println("[deps] apt-get not found; skipping automatic package installation. Assuming packages are managed manually.")
		apt = false
	}

	if apt {
		// Debian needs the non-free-firmware component; Ubuntu ships it in universe.
		if osr.isUbuntuLike() {
			ensureUbuntuUniverse()
		} else {
			ensureAptSources()
		}

		missing := []string{}
		for _, pkg := range requiredPackages {
			if !isInstalled(pkg) {
				missing = append(missing, pkg)
			}
		}

		if len(missing) == 0 {
			log.Println("[deps] All packages are satisfied.")
		} else {
			log.Printf("[deps] Missing packages: %v", missing)
			log.Println("[deps] Running apt-get update...")
			if err := runSystemCmd("apt-get", "update"); err != nil {
				log.Printf("[deps] apt-get update failed: %v (continuing)", err)
			}

			// Skip packages with no install candidate on this distro (obsolete or
			// renamed) so one unavailable package never blocks the rest.
			installable := []string{}
			for _, pkg := range missing {
				if hasCandidate(pkg) {
					installable = append(installable, pkg)
				} else {
					log.Printf("[deps] No install candidate for %q on this system; skipping.", pkg)
				}
			}

			if len(installable) > 0 {
				if err := runSystemCmd("apt-get", append([]string{"install", "-y"}, installable...)...); err != nil {
					// Fall back to one-at-a-time so a single bad package does not
					// block the others.
					log.Printf("[deps] Batch install failed (%v); retrying individually.", err)
					for _, pkg := range installable {
						if err := runSystemCmd("apt-get", "install", "-y", pkg); err != nil {
							log.Printf("[deps] Failed to install %q: %v", pkg, err)
						}
					}
				}
			}
		}

		// Fail only if a package Tala WTE cannot run without is still missing.
		stillMissing := []string{}
		for pkg := range criticalPackages {
			if !isInstalled(pkg) {
				stillMissing = append(stillMissing, pkg)
			}
		}
		if len(stillMissing) > 0 {
			return fmt.Errorf("required packages could not be installed on this system: %v", stillMissing)
		}
		log.Println("[deps] Package installation complete.")

		installOptional(osr)
	}

	ensureKernelModules()

	// Install USB cold-boot rescue units and heal any physical Wi-Fi adapter
	// wedged in the MediaTek MT7921U "patch semaphore" firmware-init state.
	EnsureWirelessRecovery()

	if err := os.MkdirAll("/var/lib/tala-wte/portals", 0o755); err != nil {
		log.Printf("[deps] Warning: failed to create portal directory: %v", err)
	}

	log.Println("[deps] Verifying runtime functionality...")
	if err := verifyFunctionality(); err != nil {
		return err
	}

	log.Println("[deps] All dependencies are satisfied.")
	return nil
}

// installOptional installs firmware packages one at a time so a single
// unavailable candidate never blocks the others or aborts startup.
func installOptional(osr osInfo) {
	missing := []string{}
	for _, pkg := range optionalFirmware(osr) {
		if !isInstalled(pkg) {
			missing = append(missing, pkg)
		}
	}
	if len(missing) == 0 {
		return
	}
	log.Printf("[deps] Installing optional firmware (best-effort): %v", missing)
	for _, pkg := range missing {
		// --no-install-recommends: firmware-misc-nonfree otherwise pulls ~48MB
		// of GPU firmware irrelevant to a wireless box.
		if err := runSystemCmd("apt-get", "install", "-y", "--no-install-recommends", pkg); err != nil {
			log.Printf("[deps] Optional package %q unavailable, skipping.", pkg)
		}
	}
}

// ensureKernelModules loads required kernel modules if they are not already present.
func ensureKernelModules() {
	loaded := loadedModules()

	for _, mod := range requiredModules {
		if loaded[mod.name] {
			log.Printf("[deps] Kernel module %s already loaded.", mod.name)
			continue
		}

		args := append([]string{mod.name}, mod.args...)
		log.Printf("[deps] Loading kernel module: modprobe %s", strings.Join(args, " "))
		if err := runSystemCmd("modprobe", args...); err != nil {
			log.Printf("[deps] Warning: failed to load kernel module %s: %v", mod.name, err)
		} else {
			log.Printf("[deps] Kernel module %s loaded successfully.", mod.name)
		}
	}
}

// loadedModules reads /proc/modules and returns a set of loaded module names.
func loadedModules() map[string]bool {
	result := make(map[string]bool)
	f, err := os.Open("/proc/modules")
	if err != nil {
		return result
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 1 {
			result[fields[0]] = true
		}
	}
	return result
}

// hasCandidate reports whether apt has an installable candidate for pkg on this
// system, so obsolete or renamed packages can be skipped instead of aborting.
func hasCandidate(pkg string) bool {
	out, err := exec.Command("apt-cache", "policy", pkg).Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Candidate:") {
			return !strings.Contains(line, "(none)")
		}
	}
	return false
}

func isInstalled(pkg string) bool {
	cmd := exec.Command("dpkg", "-s", pkg)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func runSystemCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Noninteractive: suppress debconf, apt-listchanges, and needrestart prompts.
	cmd.Env = append(
		os.Environ(),
		"DEBIAN_FRONTEND=noninteractive",
		"APT_LISTCHANGES_FRONTEND=none",
		"NEEDRESTART_MODE=a",
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %v: %w", name, args, err)
	}
	return nil
}
