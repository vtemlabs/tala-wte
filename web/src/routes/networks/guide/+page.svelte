<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, paid training, paid CTF,
  or any for-profit use requires a license from VTEM Labs. See the LICENSE file.
-->
<script lang="ts">
  import GuidePage from '$lib/components/GuidePage.svelte';

  const DOC = `## Overview

The **Networks** page is where you build and broadcast the wireless access points your exercise revolves around. A network here is not a simulation: it is a real **hostapd** access point that clients can scan for, join, and that external tools can target over the air. Each network you create captures a complete AP profile - SSID, security protocol, key material, band, channel, and the hardware radio it runs on - and you start or stop it on demand.

![The Networks page](/guide/networks.png)

The list view shows every configured network with its protocol badge, band, channel, interface, and live status. The status dot is green when a network is **running**, grey when **stopped**, and red on **error**. Use the filter field to search by SSID or protocol, the **Start** / **Stop** button to toggle a network, **Details** to open its detail page, and **Del** to remove it. The list updates in real time as networks change state.

## Why this page exists

Wireless training and assessment work needs targets that behave exactly like the real thing. Rather than mock packets or canned captures, Tala WTE stands up genuine RF beacons from your adapter so that:

- Client devices can associate with the SSID just as they would with a production AP.
- External tools (aircrack-ng, hashcat, eaphammer, reaver, and the rest) can be pointed at the network from another machine and exercise its real cryptographic handshake.
- You can demonstrate, side by side, why legacy protocols fall and why modern ones hold.

Every protocol from broken legacy WEP through WPA3-Enterprise is available, so a single console can host the full spectrum of teaching scenarios.

## Creating a network

Click **+ New Network** to open the configuration form. It is split into **Network Profile**, **Hardware**, and **Topology** sections, with the live **Protocol Guide** panel alongside.

### SSID Name

The broadcast network name. Required, and limited to **32 bytes** (the 802.11 maximum). This is the name clients see when they scan.

### Security Protocol

Choose how the network authenticates and encrypts. The form adapts to your choice: it shows a passphrase or WEP key field for protocols that need one, captive-portal options for **Open**, and routes Enterprise protocols through the preflight gate. See the protocol reference below for what each one is for.

### Passphrase

For **WPA**, **WPA2**, **WPA2 + WPS**, **WPA3**, and **WPA3-Transition**, enter a pre-shared key of **8 to 63 characters**. This is the value test clients use to join.

### WEP Key

WEP keys are special: a valid key is exactly **5 or 13 ASCII characters**, or **10 or 26 hex digits** (40-bit or 104-bit). Rather than reject anything else, the form **auto-fits** whatever you type to a valid length. A 10- or 26-digit hex string or a 5- or 13-character string is taken as-is; any other input is fitted to a 13-character (104-bit) ASCII key by truncating if it is too long or repeat-padding if it is too short. The fit is deterministic, so a given input always maps to the same key. The form then shows you the **effective key** and its type - enter that exact value on your test clients.

### Frequency Band and Channel

Pick **2.4 GHz**, **5 GHz**, or **6 GHz (Wi-Fi 6E)**, then a channel. The channel list is filtered to the legal channels for the selected band under the US regulatory domain, including the DFS channels on 5 GHz. Changing the band resets the channel to a sensible default for that band.

Bands are constrained to what the selected adapter can actually **host as an AP**. A chip can sometimes tune a band for monitor or client use but cannot beacon on it (the MT7921 radios 6 GHz but cannot host a 6 GHz AP, for example), so any band the adapter cannot broadcast on is disabled in the dropdown and labeled accordingly. If the adapter reports its AP-capable bands, a note lists exactly which bands it can host. If the chosen adapter cannot host the band you have selected, the form automatically falls back to one it can. When the adapter is unknown the form does not restrict you, and the server validates the band when you start.

### Wireless Interface

Select the radio that will broadcast the AP. Adapters are labeled with manufacturer and model where known. Tala WTE lists only real wireless hardware; virtual or simulated adapters are not supported and are not offered. If another running network is already using an interface, the form notes which adapters are in use so you can run a concurrent network on a free one. If no interfaces are detected you can type one manually (for example \`wlan0\`).

## Per-network options (Topology)

### Internet Passthrough (NAT)

When on, traffic from connected clients is NATed out through the host's uplink, so joined devices reach the internet. Turn it off to keep clients on an isolated island network with no egress.

### Client Isolation

When on, the AP blocks station-to-station traffic so clients cannot talk to each other - the same protection a guest network uses. Leave it **off** to deliberately expose the shared L2 segment and demonstrate lateral movement, ARP poisoning, and MITM between clients.

### Captive portal options (Open networks only)

For **Open** networks the form adds a **Captive Portal Sandbox** toggle. Enable it to intercept unauthenticated client traffic and serve a portal splash page, then pick a **Portal Module** from your library. A nested **Require Login (Directory / LDAP)** toggle validates the submitted username and password against the embedded directory before granting access, exactly like a corporate or ISP hotspot; failed logins are denied and recorded. See the [Captive Portals guide](/portals/guide) for building and assigning portals.

## Starting and stopping a network

Use the **Start** / **Stop** control on the list row or the **Start Network** / **Stop Network** button on the detail page.

- **Start** brings up hostapd with your configuration, begins beaconing the SSID, and (if enabled) wires NAT, isolation, and the captive portal. The status flips to **running** and the live log begins streaming.
- **Stop** tears the AP down and returns the radio. The status flips to **stopped**.

For non-Enterprise protocols the start is immediate. **Enterprise** protocols always route through the preflight gate first (see below). If a start fails, the status shows **error** and the reason surfaces as a toast and in the log.

## The network detail page

Opening a network (via its SSID link or **Details**) shows the full operational view.

### Status strip

A framed strip across the top summarizes the live state: **Status** (with the colored dot), **Protocol**, **Band**, **Channel**, and the current **Clients** count. While the network runs, these refresh automatically.

### Configuration

A read-back of the saved profile: SSID, protocol, band, channel, interface, and whether **Isolation** and **NAT** are enabled.

### Connected Clients

When stations are associated, a table lists each one by **MAC**, **IP**, and **Signal** (in dBm). The status strip's client count and this table poll live while the network runs.

### Live Log

A dark terminal pane streaming hostapd and association activity in real time. It auto-scrolls while you are at the bottom and holds position when you scroll up to read history. The header shows whether the stream is **streaming** or **idle**. Click **Pop out** to detach the log into a separate **resizable window** so you can keep it visible while you work elsewhere in the console.

The detail page polls status, clients, and logs every few seconds while running. If the connection to the server drops it keeps retrying in the background, backing off and notifying you, then confirming when the connection is restored.

## Enterprise networks and the preflight gate

**WPA2-Enterprise** and **WPA3-Enterprise** authenticate per user against a RADIUS server backed by an LDAP directory and TLS certificates, instead of a shared passphrase. Because that backend has to be in place for EAP to work, starting an Enterprise network always opens the **preflight gate** first.

The preflight checks what the Enterprise path needs - the RADIUS service, an LDAP directory with users, and server certificates - and reports anything missing. From the same dialog you can **auto-provision** the missing pieces so the network can start, rather than configuring each service by hand. Once preflight passes, the network starts like any other.

To manage these services directly, see the [RADIUS guide](/radius/guide), the [LDAP guide](/ldap/guide), and the [Certificates guide](/certificates/guide).

## The built-in Protocol Guide

Both the new-network form and the detail page carry a **Protocol Guide** panel that updates to match the selected protocol. For each one it explains what the protocol is, what it **provides** and **does not provide**, the known **vulnerability classes** (with CVE references where relevant), the **external tools** you would point at it from another device, and **recommended use cases**. It is an informational reference for planning an exercise; the actual testing is done with external tools against the live network.

## Supported protocols

The full set offered in the **Security Protocol** dropdown, and what each is used to demonstrate:

- **Open (No Auth)** - no authentication and no encryption; all traffic is cleartext. Used for captive-portal and hotspot scenarios, eavesdropping, evil-twin, and MITM demonstrations.
- **WEP (Insecure - Legacy)** - cryptographically broken since 2001; recoverable in minutes regardless of key length. Present for WEP-cracking and aircrack-ng / PTW training and to show why legacy crypto must be retired.
- **WPA (TKIP - Legacy)** - the transitional 2003 standard over RC4. Used for legacy-device compatibility testing and to demonstrate TKIP weaknesses.
- **WPA2-Personal (AES)** - the mainstream CCMP/AES standard and the most common real-world network type. Used for PSK dictionary attacks, the PMKID attack lab, and handshake-capture demonstrations.
- **WPA2 + WPS** - WPA2 with Wi-Fi Protected Setup enabled, whose PIN design is fundamentally broken. Used for Pixie Dust and PIN brute-force demonstrations.
- **WPA3-Personal (SAE)** - the modern SAE / Dragonfly standard with forward secrecy and mandatory PMF. Used to test WPA3-capable clients and demonstrate the Dragonblood side-channel.
- **WPA3-Transition (SAE+PSK)** - serves WPA2-PSK and WPA3-SAE clients on the same SSID. Used for mixed-client environments and downgrade testing.
- **WPA2-Enterprise (802.1X)** - per-user credentials via RADIUS and EAP (PEAP/TTLS or EAP-TLS), backed by FreeRADIUS and OpenLDAP. Used for corporate-network simulation, PEAP credential-harvest demonstrations, EAP-TLS mutual auth, and LDAP user-management training.
- **WPA3-Enterprise (Suite-B)** - the strongest wireless standard, adding Suite-B-192 cryptography and mandatory PMF with mutual certificate authentication. Used for high-security enterprise simulation and certificate-lifecycle training.

## Tips

- Match the target. Choose the protocol that mirrors the environment you are teaching about, and let the Protocol Guide tell you which tools to point at it.
- A wireless hardware adapter that supports AP mode is required to broadcast over the air.
- Leave **Client Isolation** off when you want to show lateral movement; turn it on to model a hardened guest network.
- Run concurrent networks by assigning each to a free interface; the form flags which adapters are already in use.
- For Enterprise networks, let the preflight gate auto-provision the backend the first time rather than wiring RADIUS, LDAP, and certs by hand.`;
</script>

<GuidePage
  title="Networks Guide"
  subtitle="Create and broadcast wireless access points across every security protocol"
  backHref="/networks"
  backLabel="Back to Networks"
  crumb="Networks"
  doc={DOC}
/>
