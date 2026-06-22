What you need to run Tala WTE: a modest Linux host, a Wi-Fi adapter that can run access-point mode, and an outbound internet uplink. Everything else (drivers, firmware, and the wireless/auth toolchain) is installed for you on first run.

See also [[Installation]], [[Quick-Start]], and [[Settings]].

## Host hardware

- CPU architecture: arm64 (aarch64) or amd64 (x86_64). Both are tested and supported, and the installer picks the matching binary by `runtime.GOARCH` (`tala-wte-linux-arm64` or `tala-wte-linux-amd64`).
- RAM and disk: modest. Tala WTE is a single Go binary with an embedded database, so a lightweight server-class host is plenty. Captures and the database live under `/var/lib/tala-wte`; give the disk enough headroom for the pcap files you intend to keep.
- Single-board computers work. Bundled firmware covers the integrated wireless on boards such as the Raspberry Pi 4 and 5, in addition to the USB adapters below.
- Root privileges are required (the service runs as `root`) to manage interfaces, hostapd, network namespaces, and firewall rules.

## Wireless adapter

Tala WTE broadcasts access points in standard AP (master) mode. It does not use monitor mode or packet injection for hosting networks, so any adapter whose driver supports AP mode for the band you want to host will work. You are standing up the target networks, not attacking them, so an ordinary adapter is enough.

The installer auto-installs firmware for the common chipset families. Verified from `internal/deps/deps.go` (`optionalFirmware`) and the adapter recognition in `internal/iface`:

- MediaTek MT76xx / MT79xx (`firmware-mediatek`) - AX-class USB cards such as the ALFA AWUS036AXM / AXML and the Panda PAU0F AXE3000 (MT7921AU), and the MT7612U-based ALFA AWUS036ACM. This is the family used to build and test Tala WTE.
- Realtek RTL8xxx (`firmware-realtek`) - USB adapters such as the RTL8812AU-based ALFA AWUS036ACH.
- Atheros (`firmware-atheros` on Debian bookworm and earlier; `firmware-ath9k-htc` on Debian trixie and later) - AR9271 cards such as the ALFA AWUS036NHA.
- Intel wireless (`firmware-iwlwifi`).
- Plus best-effort firmware for Marvell Libertas / SD8xxx (`firmware-libertas`), Broadcom (`firmware-brcm80211`), TI WiLink (`firmware-ti-connectivity`), ZyDAS ZD1211 (`firmware-zd1211`), and a non-free catch-all (`firmware-misc-nonfree`).

On Ubuntu and Ubuntu-like distros the monolithic `linux-firmware` package is installed instead, which covers all of the above.

Beyond firmware, the installer also builds out-of-tree **DKMS drivers** (it installs `dkms`, `build-essential`, and the matching kernel headers) for USB adapters that have no in-kernel driver - primarily the Realtek **RTL88xxAU** family (RTL8811AU / 8812AU / 8814AU / 8821AU / 88x2BU, e.g. the ALFA AWUS036AC / ACH / AC1200 / AWUS1900). The MediaTek MT76xx/MT79xx, Ralink RT3xxx, and Atheros AR9271 families are in-kernel and need only firmware. The common DKMS driver set is installed best-effort (each package skipped if it has no candidate on your distro). Verified from `internal/deps/deps.go` (`optionalDrivers`).

If you plug in an adapter with no driver support, the installer and the app both call it out (a warning at install time and an "Unsupported" tag with a **Heal** button in [[Settings]]) so you know to install a driver before that radio can be used.

### Recommended

The Panda Wireless PAU0F AXE3000 (MediaTek MT7921AU) is the adapter used to build and test Tala WTE, and a budget-friendly pick. It runs the same MT7921AU chipset as the pricier ALFA AWUS036AXM. A tri-band (2.4 / 5 / 6 GHz) Wi-Fi 6E USB 3.0 card, it hosts WPA2 and WPA3 access points on 2.4 and 5 GHz and joins all three bands as a client.

When to care about the chipset: pick a MediaTek MT79xx card if you want to host WPA3-SAE and 5 GHz networks; older chipsets (for example RT3070-based 2.4 GHz-only cards) are fine for legacy and WPA2 2.4 GHz labs but are flagged as limited in the interface picker.

## Operating system

Tala WTE auto-installs its dependencies on apt-based Linux only. Verified from `internal/deps/osdetect.go`, the recognized apt family is: Debian, Ubuntu, Kali, Raspberry Pi OS (raspbian), Linux Mint, Pop!_OS, elementary OS, Zorin, and KDE neon, plus any distro whose `ID_LIKE` chains to one of these.

Tested platforms:

| Distribution | Version | Status |
| --- | --- | --- |
| Debian | 13 (Trixie) headless | Tested, recommended |
| Ubuntu | 24.04 LTS | Tested |
| Ubuntu | 22.04 LTS | Tested |
| Ubuntu | 26.04 LTS | Runs, not recommended (kernel Wi-Fi/mt76 instability) |
| Kali Linux | 2026.1 | Tested, not recommended |

**A headless Debian 13 (Trixie) server is the recommended platform** - no desktop environment, and the fewest surprises with USB Wi-Fi drivers and the toolchain. Ubuntu 24.04 LTS is also a solid choice. Ubuntu 26.04 will run, but is **not recommended**: its newer kernel has USB Wi-Fi driver (mt76) instability that can wedge a radio under sustained load. Kali works but is not recommended. On a non-apt distro the auto-installer is skipped and you must install the tools below with your own package manager; Tala WTE still verifies every core capability at startup and reports anything missing rather than failing later.

## Auto-installed dependencies

On first run (and on every boot, idempotently), Tala WTE installs and verifies its dependencies. Verified from `internal/deps/deps.go`:

Required packages (`requiredPackages`):
`hostapd`, `dnsmasq`, `slapd`, `ldap-utils`, `freeradius`, `freeradius-ldap`, `freeradius-utils`, `tshark`, `iptables`, `iproute2`, `curl`, `pciutils`, `usbutils`, `hwdata`, `rfkill`, `iw`, `wpasupplicant`, `tcpdump`, `psmisc`, `procps`, `git`.

Of these, the install fails only if a critical package is still missing (`criticalPackages`): `hostapd`, `dnsmasq`, `iw`, `iproute2`, `iptables`, `tshark`. The rest are best-effort: a package that is obsolete, renamed, or absent on a given release is skipped rather than aborting the whole install, and a batch failure falls back to per-package installs so one bad package never blocks the others.

Firmware is installed automatically (best-effort, one package at a time) as listed under Wireless adapter above. On Debian/Kali the installer enables the `non-free-firmware` (and `non-free`) component in `/etc/apt/sources.list` so the firmware packages are available; on Ubuntu-like distros it enables the `universe` repo instead.

After installation Tala WTE verifies the actual runtime binaries are present (not just the package names) so a renamed or split package on any distro surfaces as a clear capability error. Core capabilities checked: hostapd, dnsmasq, `iw`, `wpa_supplicant`, `ip`, `iptables`, `tshark`/`capinfos`, OpenLDAP (`slapd`/`slapadd`/`ldapsearch`/`ldapadd`), and FreeRADIUS. `tcpdump` and procps/psmisc are non-critical (a missing one degrades a feature rather than failing the install).

## Network

- Outbound uplink: an internet-facing interface is needed for client traffic generation, NAT passthrough on networks with Internet Passthrough enabled, and for in-app software updates. Tala WTE auto-detects the uplink; you can override it under Settings -> Uplink Interface (Internet).
- Console exposure: the web console is served over HTTPS on port 8443 (port 80 redirects to it). Do not expose 8443 or the other services to inbound connections from the internet or untrusted networks.
- Isolation: Tala WTE is a vulnerable-by-design training range that deliberately runs insecure networks and weak credentials. Run it only on a dedicated, isolated lab network, never on a production or corporate LAN, and transmit only on Wi-Fi channels and a regulatory domain you are authorized to use (set it under [[Settings]]).
