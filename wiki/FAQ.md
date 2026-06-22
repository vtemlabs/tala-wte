# FAQ

Short answers to the most common questions about Tala WTE. Each links to a page with more detail.

## Basics

### What is Tala WTE?
Tala WTE (Wireless Training Environment) turns a single Linux host with a Wi-Fi adapter into a complete wireless lab. You stand up target networks, captive portals, and enterprise authentication, then practice wireless penetration testing against them. Everything (web console, database, captive portal engine, OpenLDAP, FreeRADIUS, a certificate authority) ships in one Go binary. See [[Architecture]].

### Is it free?
Free for personal and non-profit use. Commercial, for-profit, and government use require a license. A CTF at a paid-admission conference counts as for-profit and needs a license. See [[Security-and-License]].

### What is the difference between Tala WTE and TALA?
Tala WTE is the open training and development range you learn and rehearse on. TALA is VTEM Labs' professional wireless penetration testing platform used in the field; it is delivered through the VTEM Labs ARROW program and, for the tactical multi-modal variant, to authorized government, DoW, and LEO operators. In short: Tala WTE trains, TALA hunts. See [[Architecture]] and [[Security-and-License]].

## Hardware and platform

### What hardware or Wi-Fi adapter do I need?
A clean Linux host (arm64 or x86_64) and a Wi-Fi adapter whose driver supports AP (master) mode for the bands you want to host. Tala WTE broadcasts standard access points; it does not use monitor mode or injection for the AP role, so any AP-capable adapter works. The recommended adapter is the Panda Wireless PAU0F AXE3000 (MediaTek MT7921AU), the same chipset as the ALFA AWUS036AXM. Integrated Wi-Fi on boards like the Raspberry Pi 4 and 5 is also supported. See [[Installation]].

### Which operating systems are supported?
A clean apt-based Debian or Ubuntu host. **Recommended: a headless Debian 13 (Trixie) server** - no desktop environment, and the fewest surprises with USB Wi-Fi drivers. Ubuntu 24.04 LTS is also a solid choice; Ubuntu 22.04 is tested too. Ubuntu 26.04 runs but is **not recommended** - its newer kernel has USB Wi-Fi driver (mt76) instability that can wedge a radio under sustained load. Kali Linux 2026.1 is tested but not recommended. Tested on both arm64 and x86_64. See [[Installation]].

### Do I need internet?
You need an outbound internet uplink for client traffic generation and for in-app updates. You do not need, and should not have, inbound exposure. You can also turn Internet Passthrough off per network for a sealed, local-only exercise. See [[Security-and-License]] and [[Settings]].

### Can it run in a VM?
Yes. Note that on a USB-passthrough VM, a software USB reset cannot recover a wedged adapter, because the hypervisor provides the virtual xHCI. If Heal fails on an adapter in a VM, physically replug it (or detach and re-attach it in the hypervisor). See [[Troubleshooting]].

## Setup and login

### Is there a default login?
No. Tala WTE never auto-provisions an administrator and never prints credentials to a terminal. On first run you create the one admin account in the browser, gated by a one-time setup token written to the host log, and you must acknowledge the license first. See [[Installation]] and [[Troubleshooting]].

### Why do I get an HTTPS security warning?
The console is served over HTTPS on port 8443 using a self-signed certificate the binary generates for itself. Browsers do not trust a self-signed cert, so they warn. Proceed past the warning; this is expected for a lab appliance. See [[Troubleshooting]] and [[Installation]].

### How do I reset or start over?
Uninstall the service with `sudo tala-wte uninstall`. The database under `/var/lib/tala-wte` is preserved across reinstalls, so a plain uninstall and reinstall keeps your data. Add `--purge` to also delete the database and all captures, which gives you a clean first-run setup screen again. See [[Installation]].

## Networks and capture

### How many networks can I broadcast at once?
One AP-capable adapter hosts one network at a time. To broadcast more networks simultaneously, add more adapters; each running network claims its own adapter. See [[Architecture]] and [[Troubleshooting]].

### Will it touch my real or home network?
It should not be on your real network at all. Run it on an isolated lab segment. Internet passthrough NATs client traffic out the uplink interface, so connected clients reach the internet through that uplink; turn passthrough off on a network to wall it off. Never run Tala WTE on a production or corporate LAN. See [[Security-and-License]].

### How do I capture a WPA handshake?
Start a packet capture on the running network, then make a client associate and reassociate so the four-way handshake is on the air. From a client or the Pack, use the Handshake capture profile (reconnect cycling), which deauthenticates and reassociates on a schedule so a fresh handshake is produced each cycle. A wireless (802.11 monitor) capture records the raw management frames. See [[Troubleshooting]] and [[The-Pack]].

### Why does my capture show 0 packets?
That is not a bug. A network-layer capture only records traffic that actually crosses the AP, so an idle network with no clients captures nothing. Connect a client or deploy a Pack member with traffic generation on that network and the count climbs. See [[Troubleshooting]].

### Why is Captured Data empty?
The captive portal only records what a client actually submits. Until a client (or a Pack member) connects and submits the portal form, there is nothing to show. See [[Troubleshooting]] and [[The-Pack]].

## Adapters

### Why will my adapter not work or show Unsupported?
"Unsupported" means a USB wireless device is present but has no working radio (no ieee80211 phy), usually because its driver or firmware is not loaded. If it is a recognized chipset, the driver and firmware are present and it just did not initialize, so replug it (common on USB-passthrough VMs). For an unrecognized device, install the driver for that adapter. The Settings page offers a Heal button that performs a USB reset to recover a wedged adapter. See [[Troubleshooting]] and [[Settings]].

### What does the "radio wedged" badge on a Pack member mean?
The member is reachable, but its wireless driver stopped responding (a driver hang, common on mt76/MediaTek radios after a rapid stop/start). It needs the adapter power-cycled or physically replugged; it is a hardware reset, not a software error. See [[Troubleshooting]] and [[The-Pack]].

## The Pack and updates

### How do I add more client machines (the Pack)?
A server can act as a pack leader and drive client members. On each client, copy its agent key from Settings, then register the member on the leader by its address and key (or discover it on the LAN). The leader then deploys a network config and a traffic profile to each member. See [[The-Pack]] and [[Settings]].

### How do I update?
Updates are in-app only. Under Settings, Software Updates shows the installed and latest versions; one click downloads the architecture-matched binary, verifies it against the release checksums, swaps it in place, and restarts the service. Running a pack? The Pack page has an Update all members action that pushes the same update to every reachable member. Do not hand-build or copy binaries to update. See [[Settings]] and [[The-Pack]].

### Why is the update button disabled or failing?
Development (untagged local) builds disable in-place updates, since the updater will not replace a binary it did not release. A check that cannot reach GitHub fails softly and still shows your current version. See [[Troubleshooting]] and [[Settings]].

## Data and safety

### Where is my data stored?
Under `/var/lib/tala-wte` on the host: the PocketBase database, captures, portals, the LDAP directory, RADIUS config, and the certificate authority all live there. It is preserved across reinstalls and deleted only with `uninstall --purge`. See [[Installation]].

### Is my data and credentials safe?
Treat the box as untrusted lab equipment. Tala WTE is vulnerable by design: it broadcasts attackable networks, harvests portal submissions, and ships weak credentials for cracking practice. Keep it on an isolated network, never expose the console to the internet, and do not store anything sensitive on it. See [[Security-and-License]].

### Can I use this at a paid-admission conference CTF?
No, not for free. A CTF or activity held at, or as part of, a conference or venue that charges paid admission or registration is for-profit use and requires a commercial license, regardless of the event's non-profit status. See [[Security-and-License]].

## Other

### Do enterprise (WPA-Enterprise) networks need setup before they start?
They need a CA, a server certificate, LDAP users, and FreeRADIUS running. You do not have to do this by hand: when any of it is missing, the Start button becomes Auto-provision & Start, which bootstraps the whole enterprise stack and then starts the network. See [[Troubleshooting]].

### Can Tala WTE be used as a honeypot?
The same pieces make it usable as a passive wireless decoy: it broadcasts an inviting, never-silent network and records associations, captive-portal submissions, the live client list, and packet captures. It is built as a training range rather than a hardened sensor, so active-attacker alerting is beyond what ships today, but the recording groundwork is there. See [[Architecture]].
