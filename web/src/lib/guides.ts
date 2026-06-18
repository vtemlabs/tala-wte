// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government use
// require a license from VTEM Labs. See the LICENSE file.

// Shared in-app guide content. Each guide renders both full-page (the /*/guide
// routes) and in the floating, adjustable Guide window, so a guide can be
// referenced side by side while adjusting settings and running tasks.

export interface Guide {
	title: string;
	subtitle?: string;
	doc: string;
}

export const GUIDES: Record<string, Guide> = {
	networks: {
		title: 'Networks Guide',
		subtitle: 'Create and broadcast wireless access points across every security protocol',
		doc: `## Overview

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

![The new-network form](/guide/networks-new.png)

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

![The network detail page](/guide/network-detail.png)

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

![The enterprise preflight gate](/guide/networks-preflight.png)

**WPA2-Enterprise** and **WPA3-Enterprise** authenticate per user against a RADIUS server backed by an LDAP directory and TLS certificates, instead of a shared passphrase. Because that backend has to be in place for EAP to work, starting an Enterprise network always opens the **preflight gate** first.

The preflight checks what the Enterprise path needs - the RADIUS service, an LDAP directory with users, and server certificates - and reports anything missing. From the same dialog you can **auto-provision** the missing pieces so the network can start, rather than configuring each service by hand. Once preflight passes, the network starts like any other.

To manage these services directly, see the [RADIUS guide](/radius/guide), the [LDAP guide](/ldap/guide), and the [Certificates guide](/certificates/guide).

## The built-in Protocol Guide

![The Protocol Guide panel](/guide/networks-protocol.png)

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
- For Enterprise networks, let the preflight gate auto-provision the backend the first time rather than wiring RADIUS, LDAP, and certs by hand.`
	},
	captures: {
		title: 'Packet Captures Guide',
		subtitle: 'Passive wireless and network-layer capture and analysis',
		doc: `
## What this page is for

The Packet Captures page records traffic from your training networks so you can
observe and analyze exactly what connected clients are sending. Passive capture
is the foundation of most wireless and network assessment work: it lets you
prove that an open or weakly secured network exposes data in cleartext,
inspect protocols and credentials as they cross the air or the wire, and
collect WPA handshakes for later analysis.

Captures here are **passive**. You are recording traffic that already exists on
a network you operate for training. Nothing is injected, modified, or
re-transmitted. The result of every session is a standard \`.pcap\` file you can
open in Wireshark, tshark, or any other packet tool.

![The Packet Captures page](/guide/captures.png)

> Only capture on networks you own or are explicitly authorized to test. This
> tool is built for the training networks you stand up in Tala WTE, not for
> arbitrary infrastructure.

---

## The two capture layers

When you start a capture you choose a **Layer**. This decides what kind of
traffic is recorded and which interface the capture runs on. The two options
map exactly to the choices in the Start New Capture form.

### Network (IP layer)

Network captures run \`tshark\` directly on the access point interface and record
traffic at the **IP layer** and above: TCP, UDP, ARP, DNS, HTTP, TLS, and so
on. This is the view from inside the network, after a client has associated and
been issued an address.

Use the Network layer when you want to:

- Show that an open or weak network carries credentials and page content in
  cleartext over HTTP, FTP, Telnet, or unencrypted DNS.
- Inspect what applications and devices actually talk to once they are online.
- Capture ARP, DHCP, and DNS activity to map the clients on the network.

When the selected network is **running**, a Network capture automatically records
the traffic of its **connected clients**. Each running network owns an isolated
network namespace, and the capture is run inside it, so a real device joined to
the access point (a phone, laptop, or IoT client) has its DNS lookups, web
requests, and other traffic captured exactly as the network sees it. If the
network is not running, the capture records on the host interface instead.

### Wireless (802.11)

Wireless captures record raw **802.11 frames** from a **monitor-mode**
interface. This is the radio view: beacons, probe requests and responses,
association and authentication frames, and the EAPOL frames that make up a WPA
four-way handshake.

Use the Wireless layer when you want to:

- Collect a WPA/WPA2 handshake for offline analysis.
- Observe management and control frames, probe behavior, and which SSIDs
  clients are searching for.
- Work below the IP layer, before or without any client association.

The interface you select for a Wireless capture must support and be placed in
monitor mode. Pick an adapter dedicated to capture rather than the one currently
serving the access point.

---

## Starting a capture

![Start a new capture](/guide/captures-start.png)

The Start New Capture panel has four inputs. Fill them in top to bottom, then
press **Start Capture**.

1. **Network** - Select the training network whose traffic you want to record.
   The dropdown lists each network by SSID along with its current status. The
   selected network is recorded with the session so captures stay organized by
   target. See the [Networks guide](/networks/guide) for how to create and run
   the networks that appear here.
2. **Layer** - Choose **Network (IP layer - tshark on AP interface)** or
   **Wireless (802.11 - monitor mode interface)** as described above.
3. **Interface** - Choose the capture interface. If Tala WTE has detected
   wireless interfaces they appear in a dropdown; otherwise you can type one in
   by hand, for example \`wlan0\`. For a Network capture this is the AP
   interface; for a Wireless capture this is a monitor-mode interface.
4. **BPF Filter (optional)** - Narrow what gets recorded. Leave this blank to
   capture everything on the interface. See below for syntax and examples.

The Start Capture button stays disabled until both a Network and an Interface
are selected. Once you start, the new session appears in the Capture Sessions
table with a status of \`running\`.

---

## BPF capture filters

The optional filter field uses **BPF** (Berkeley Packet Filter) syntax, the
same capture-filter language used by tcpdump and Wireshark's capture filters. A
BPF filter decides **what is written to disk in the first place**: anything that
does not match is never recorded, which keeps the pcap small and focused.

Common examples:

- \`port 80\` - only traffic to or from TCP/UDP port 80 (HTTP).
- \`tcp port 443\` - only HTTPS / TLS traffic.
- \`udp port 53\` - only DNS queries and responses.
- \`host 10.0.0.1\` - only traffic to or from a single host.
- \`arp\` - only ARP frames, useful for mapping the local segment.

You can combine terms with \`and\`, \`or\`, and \`not\`, for example
\`tcp port 80 and host 10.0.0.1\`. Because a BPF filter is applied at capture
time, anything it excludes is gone for good. When in doubt, capture broadly and
filter later in your analysis tool.

---

## Managing capture sessions

Every session is listed in the Capture Sessions table with its SSID, Layer,
Interface, packet count, and status. The actions available on each row depend on
whether the session is still running.

- **Status** - A session is either \`running\` (actively recording, shown with a
  lit status dot) or \`stopped\`. The **Packets** column shows how many packets
  have been recorded so far.
- **Stop** - While a session is \`running\`, the only action is **Stop**, which
  ends the capture and finalizes the pcap file.
- **View** - Once a session is stopped, **View** opens the built-in PCAP viewer
  (covered below).
- **Download** - **Download** retrieves the resulting \`.pcap\` file so you can
  open it in your analysis tool of choice.
- **Del** - Deleting a stopped session removes the **record** from the session
  list. The underlying pcap file is **preserved on disk**, so deleting here only
  cleans up the table and does not destroy captured data.

---

## The built-in PCAP viewer

![The PCAP analysis view](/guide/capture-analysis.png)

You do not need to leave the app to review a capture. **View** on any stopped
session opens an in-app viewer with two tabs:

- **Analysis** - a summary of the capture: total packets, duration, and size; the
  protocol mix; the top talkers; HTTP requests; the TLS server names (SNI) that
  clients connected to; DNS queries; HTTP user agents; and, most importantly, any
  **cleartext credentials** recovered from the traffic (HTTP Basic auth and form
  posts). This is the payoff of capturing on an open or weak network.
- **Packets** - a Wireshark-style packet list (number, time, source, destination,
  protocol, length, and info) with a **display filter** box. Display filters use
  Wireshark's syntax (for example \`http\`, \`dns\`, or \`ip.addr==10.0.0.1\`), which
  is different from the BPF capture filter used when starting the capture.
  Selecting a packet shows its full dissection.

The viewer also has a **Download pcap** button for opening the same capture in an
external tool.

## Analyzing the pcap externally

The download is a standard pcap. Open it in **Wireshark** for a full graphical
analysis, or process it from the command line with **tshark**:

\`\`\`
tshark -r capture.pcap -Y "http.request"
\`\`\`

A key point: Wireshark **display filters** are a different language from the
**BPF capture filters** used on this page. Capture filters (BPF) decide what
gets recorded; display filters decide what you see in an already-recorded file,
and they use protocol field names.

Useful Wireshark display filters:

- \`http.request\` - show outgoing HTTP requests, including URLs and headers.
- \`dns\` - show DNS queries and responses.
- \`eapol\` - isolate the EAPOL frames of a WPA handshake (Wireless captures).
- \`ip.addr == 10.0.0.1\` - everything to or from one host.
- \`tcp.port == 443\` - all TLS / HTTPS traffic.

For a Network-layer capture on an open or weak SSID, filtering on
\`http.request\` quickly surfaces cleartext credentials and page content. For a
Wireless-layer capture, the \`eapol\` filter confirms you have all four frames of
a handshake before you move on to offline analysis.

---

For setting up and running the networks you capture from, see the
[Networks guide](/networks/guide).
`
	},
	portals: {
		title: 'Portal Guide',
		subtitle: 'How to create, edit, clone, and deploy captive portals',
		doc: `## Overview

A **captive portal** is the splash page a client sees after joining an open Wi-Fi network, before it can reach the internet. In Tala WTE a portal is assigned to an **Open** network; when a client connects, all of its web traffic is intercepted and redirected to the portal until it submits the form. Every value the client enters is captured to **Captured Data**, and portals can optionally validate those credentials against the embedded directory, behaving like a real credentialed hotspot.

Use portals to demonstrate how convincingly a rogue access point harvests credentials and personal information from a fake login page.

## The portal library

The **Captive Portals** page is your template library. Each card is a portal you can preview, edit, clone, or assign to a network.

![The portal library](/guide/gallery.png)

- **Built-in** templates ship with Tala WTE and model realistic venues: coffee shops, hotels, corporate guest pages, airports, in-flight, and ISP hotspots. They are managed by the app and kept current automatically.
- **Custom** templates are ones you create, upload, or clone. Editing a built-in clones it first, so the originals stay intact.
- Use the **category chips** to filter the library, and the **Captured Data** link at the top to jump to everything harvested so far.

## Creating a portal

![Four ways to add a portal](/guide/portals-actions.png)

There are four ways to add a portal. All of them land you in the editor, where you can refine the HTML and preview it live.

### 1. Clone a built-in template

The fastest start. On any built-in card, click **Clone**. You get an editable copy named "<template> (copy)" that you can rename and customize without touching the original.

### 2. Start from scratch

Click **+ New Portal**. You begin with a blank editor, or a starter skeleton via **Insert Starter HTML**. Paste or write any HTML; Tala WTE auto-wires the first login form to capture and (optionally) authenticate, so you do not have to hand-edit form actions.

### 3. Clone from a live URL

Click **Clone from URL** and paste the address of a real sign-in page, for example a vendor hotspot login. Tala WTE fetches the page, inlines its assets, and imports it as a portal. Review the result in the editor before using it.

### 4. Upload a template

Click **Upload Template** to import an \`.html\` file or a \`.zip\` bundle (HTML plus its images, CSS, and JS). Bundles are served as a small static site, so multi-file portals keep their assets and links.

## Editing a portal

Opening a portal shows the editor: the **HTML Source** on the left and a **Live Preview** on the right that updates as you type. Edit the markup, then click **Save**.

![The portal editor](/guide/editor.png)

You do not need to wire forms by hand. On save and on serve, Tala WTE normalizes the portal: it points the first form at the capture endpoint, tags recognized username and password fields, and adds a redirect so the client lands on a normal page after submitting. Field names like \`username\`, \`email\`, \`member_id\`, \`password\`, and \`pin\` are detected automatically.

## Capturing credentials and PII

Every field a client submits is recorded to **Captured Data** with the client's MAC address, IP, and browser. This is the payoff of the exercise.

![Captured data](/guide/captured.png)

When a portal is assigned to a network with **Validate credentials** enabled, Tala WTE checks the submitted username and password against the embedded LDAP directory before granting access, exactly like a real credentialed hotspot. The result (\`success\` or \`fail\`) is stored alongside each submission, and failed logins are re-prompted instead of being waved through.

## Assigning a portal to a network

![Assign a portal in the network form](/guide/portals-assign.png)

A portal does nothing until it is attached to a running **Open** network.

1. Create or edit a network and set the **Security Protocol** to **Open**.
2. Enable the captive portal and choose a template from the list.
3. Optionally turn on **Validate credentials** to authenticate against the directory.
4. Start the network. Connecting clients are redirected to your portal until they submit the form.

## Legal links work

The built-in templates link to **Terms of Service**, **Acceptable Use Policy**, and **Privacy Policy**. These resolve to real generic policy pages served by the portal, so the splash feels complete and legitimate to a connecting client. Each page links back to the sign-in screen.

## Tips

- Preview before you deploy: use the **Preview** action on any card, or the live preview in the editor.
- Keep credential field names conventional so auto-capture and validation work without manual wiring.
- Clone, do not edit, when you want a variation of a built-in template.
- Match the venue. The closer your portal looks to a network the client expects, the more convincing the exercise.`
	},
	ldap: {
		title: 'LDAP Directory Guide',
		subtitle: 'The embedded directory that backs enterprise authentication',
		doc: `
## What this page is

The LDAP Directory is the embedded user store that backs enterprise wireless
authentication on this appliance. It runs a local OpenLDAP server (\`slapd\`)
that holds the accounts, groups, and credentials your simulated clients will
authenticate against.

![The LDAP Directory page](/guide/ldap.png)

In a WPA2-Enterprise or WPA3-Enterprise (802.1X) network, the access point does
not check the password itself. It hands the credentials to RADIUS, and RADIUS
validates them against a backend user store. On this appliance that backend is
this directory. The same directory also backs the optional credential-validation
mode of Open-network captive portals, where a harvested username and password
are checked for validity instead of being blindly accepted.

In short: edit a user here and you change who can join your enterprise networks
and whose harvested credentials will be accepted by a portal. See the
[RADIUS guide](/radius/guide) for how the authentication path is wired together,
and the [Networks guide](/networks/guide) for where enterprise networks are
configured.

## Directory status

The header shows a \`slapd running\` or \`slapd stopped\` badge. Authentication
only works when the badge reads **running**. If it reads stopped, no enterprise
client and no portal credential check will succeed.

Below the header a status strip reports the directory's coordinates:

- **Base DN** - the root of the directory tree, for example
  \`dc=acmecorp,dc=local\`. Every user and group lives under this DN.
- **Bind DN** - the administrative account RADIUS and the portal use to read the
  directory, shown as \`cn=admin,<base DN>\`.
- **Port** - the LDAP service port, \`3389\`. This is a local, appliance-internal
  service.
- **Users** - the current count of accounts in the directory.

## Directory Provisioning

![Directory provisioning](/guide/ldap-provision.png)

Provisioning **wipes and rebuilds the entire directory** with a fresh set of
generated users, groups, and credentials. Use it to stand up a believable
corporate directory in one click instead of adding accounts by hand. Every
provisioning action asks for confirmation first, because it destroys the
existing contents.

There are three ways to provision:

### Reset to Default (ACME Corp)

Rebuilds the directory as the standard demo company: **ACME Corp**, domain
\`acmecorp.local\`, **15 users**, using the realistic password mix described
below. This is the known-good baseline to return to when a directory has been
edited beyond recognition.

### Generate Random Company

Builds a fresh directory for a randomly generated company name and domain, with
a generated set of users and credentials. Use it when you want a different,
unfamiliar directory for an exercise rather than the always-the-same ACME Corp.

### Custom

Opens an inline form so you can specify the directory yourself:

- **Company Name** - required. Used for the organization and in generated names.
- **Email Domain** - required, for example \`contoso.local\`. Generated user
  email addresses use this domain.
- **Users** - how many accounts to generate, from 1 to 50.

A toggle, **All Strong Random Passwords**, controls how credentials are
generated:

- **Off (recommended)** - a realistic corporate password mix: roughly 40 percent
  weak (think \`Password1!\`, \`Welcome123\`), roughly 30 percent semi-personal
  (a first name plus a year), and roughly 30 percent strong random. This is what
  you want for a credible cracking or harvesting exercise, because real
  directories contain weak passwords.
- **On** - every user gets a unique 12-character random password. Choose this
  only when you specifically want a directory full of password-manager-grade
  accounts.

Press **Provision** to build the directory.

### Provisioning results

After any provisioning run, a result table lists the generated accounts with
their **UID**, **name**, **email**, and **password** in plaintext. This is your
record of the credentials that now exist, so copy down anything you need for the
exercise before navigating away.

## Managing users

![The Users tab](/guide/ldap-users.png)

Below the provisioning panel, the **Users** tab lists every account and lets you
add or remove them individually.

### Adding a user

The add-user row has five fields:

- **UID** (required) - the login name, for example \`jdoe\`. This is what a client
  or portal submits as the username.
- **CN (Full Name)** (required) - the display name, for example \`John Doe\`.
- **SN (Last Name)** - the surname, for example \`Doe\`.
- **Email** - the user's email address, for example \`jdoe@tala.wte\`.
- **Password** (required) - the credential the user authenticates with.

UID, CN, and Password are mandatory; the **Add User** button stays disabled until
all three are filled. Submitting adds the account to the live directory
immediately, so a client can authenticate with it right away.

### Viewing and copying passwords

Each row shows the user's password masked behind dots. For accounts stored as
plaintext you can:

- **Show / Hide** - reveal or re-mask the password in place.
- **Copy** - copy the password to your clipboard so you can paste it into a
  client supplicant or portal form.

Passwords that \`slapd\` stored as a one-way hash (rendered as \`(hashed)\`)
cannot be revealed or copied, because the original value is not recoverable.
Provisioned and manually added accounts are stored as plaintext so they remain
usable for exercises.

> Clipboard copy can fail in a non-secure browser context. If the copy button
> reports an error, reveal the password with Show and copy it manually.

### Deleting a user

The **Del** action on a row removes that account from the directory after a
confirmation prompt. A deleted user can no longer authenticate to enterprise
networks or pass a portal credential check.

## Groups

![The Groups tab](/guide/ldap-groups.png)

The **Groups** tab lists the directory's groups and lets you create new ones.
Enter a **Group CN** (for example \`wifi-users\`) and press **Create Group**.
Each group card shows the group's common name, its full DN, and any members it
contains. Groups are useful for organizing accounts and for any policy that
keys off group membership.

## Test Auth

![The Test Auth tab](/guide/ldap-testauth.png)

The **Test Auth** tab is a quick credential check that runs a real bind against
the directory, exactly the way RADIUS does during 802.1X. Use it to confirm a
username and password actually work **before** a client tries them, so you can
tell a typo apart from a genuine authentication failure.

1. Enter a **Username (UID)** and **Password**.
2. Press **Test Authentication**.

A green **Authentication Successful** result confirms the credentials bind and
shows the matched user DN. A red **Authentication Failed** result means the bind
was rejected, and the message explains why (wrong credentials, no such user, or
the directory being unreachable because \`slapd\` is stopped).

## How it ties together

This directory is the single source of truth for "who is allowed in" across the
appliance:

- **Enterprise wireless (802.1X)** - WPA2-Enterprise and WPA3-Enterprise
  networks authenticate clients through RADIUS, and RADIUS validates the
  submitted credentials against this directory. A user that exists and binds
  here is a user that can join the network. See the [RADIUS guide](/radius/guide).
- **Open-network captive portals** - a portal configured to validate harvested
  credentials checks the captured username and password against this directory,
  so only credentials that are real (present and correct in the directory) are
  accepted. See the [Networks guide](/networks/guide).

Provision a believable directory, verify a couple of accounts on the Test Auth
tab, and your enterprise networks and credential-checking portals are ready to
exercise against real, known credentials.
`
	},
	radius: {
		title: 'RADIUS Guide',
		subtitle: 'The 802.1X authentication server for WPA-Enterprise networks',
		doc: `## Overview

**RADIUS** (Remote Authentication Dial-In User Service) is the authentication server that stands behind every **WPA2-Enterprise** and **WPA3-Enterprise** Wi-Fi network. Tala WTE runs **FreeRADIUS 3.x** as that server. When an enterprise access point needs to decide whether a client is allowed onto the network, it does not check the password itself. It hands the question off to RADIUS over the **802.1X** framework, and RADIUS answers with a yes or no.

This page is where you configure how that server speaks **EAP** (Extensible Authentication Protocol) to clients, which credential backend it trusts, and the shared secret it uses to talk to your access points. Use it to stand up a realistic enterprise authentication target for training and assessment.

![The RADIUS page](/guide/radius.png)

> **Why this matters:** Enterprise Wi-Fi is the most common real-world deployment in corporate environments. To demonstrate attacks like evil-twin RADIUS impersonation, credential relay, or weak inner-method downgrades, you first need a working 802.1X server. That is what this page provides.

## Status and ports

The badge in the page header shows whether **FreeRADIUS** is currently **running** or **stopped**. The stat strip below it lists the three endpoints that make enterprise authentication work:

- **RADIUS Port 1812 (Authentication)** - the UDP port that access points send **Access-Request** packets to. This is the front door for every login attempt.
- **Accounting Port 1813 (Accounting)** - the UDP port used for session accounting (start, interim, and stop records). It tracks who connected, for how long, and how much data moved.
- **LDAP Backend 127.0.0.1:3389 (OpenLDAP / slapd)** - the directory that actually stores the user accounts and passwords. FreeRADIUS does not keep its own user database here; it validates every credential against this **OpenLDAP** instance.

## EAP configuration

![EAP configuration](/guide/radius-eap.png)

The **EAP Configuration** panel controls how clients prove who they are. There are three settings.

### Default EAP Type

EAP is a wrapper that carries the real authentication exchange. The outer EAP type decides how that exchange is protected. Tala WTE offers four:

- **EAP-PEAP (Recommended)** - Protected EAP. The server presents a certificate and builds a **TLS tunnel**, then runs a simpler inner authentication (typically MSCHAPv2) inside that tunnel. The client does not need its own certificate, which is why PEAP is the most widely deployed enterprise method. This is the recommended default.
- **EAP-TLS (Certificate Auth)** - Mutual certificate authentication. **Both** the server and the client present certificates, and there is no password at all. It is the strongest method, but it requires that every client be issued a client certificate first (see the Certificates page).
- **EAP-TTLS** - Tunneled TLS. Like PEAP, it builds a server-authenticated TLS tunnel, but it is more flexible about what runs inside and can carry legacy inner methods such as PAP or CHAP safely.
- **EAP-FAST** - Flexible Authentication via Secure Tunneling. A Cisco-originated method that establishes the tunnel using a **PAC** (Protected Access Credential) instead of, or in addition to, a server certificate.

### Inner Authentication

Inside the protected tunnel, the actual credential check runs. The inner method determines how the password is exchanged:

- **MSCHAPv2** - Microsoft Challenge-Handshake. The default partner for PEAP, and what Windows, macOS, iOS, and Android expect out of the box. It performs a challenge-response so the password is never sent in clear form inside the tunnel.
- **PAP** - Password Authentication Protocol. The password is passed in clear text, but only inside the TLS tunnel. PAP is required when the backend must see the raw password to validate it against the directory.
- **CHAP** - Challenge-Handshake. An older challenge-response scheme retained for compatibility with legacy supplicants.
- **GTC** - Generic Token Card. Designed for one-time passwords and token-based credentials, where the client is prompted for a value rather than reusing a stored password.

> The inner method only applies to tunneled outer types (PEAP, TTLS, FAST). **EAP-TLS** uses certificates and has no inner password step.

### Shared Secret

The **Shared Secret** is the password between the **access point** and the **RADIUS server** itself, not a user credential. Every RADIUS packet between hostapd and FreeRADIUS is authenticated with it, so the two must match exactly. Leave the field blank to use the generated secret that Tala WTE provisions automatically, or set your own to mirror a specific environment.

Click **Save Configuration** to apply the EAP type, inner method, and shared secret.

## The authentication chain

![The authentication chain](/guide/radius-chain.png)

The **Authentication Chain** panel diagrams the full end-to-end flow exactly as a client experiences it. From association to a connected session, the path is:

\`\`\`
Wi-Fi Client
  |  EAP-PEAP / MSCHAPv2
hostapd (AP)
  |  RADIUS Access-Request  -> :1812
FreeRADIUS
  |  LDAP bind              -> :3389
OpenLDAP
  |  Access-Accept + MSK
4-Way Handshake -> Connected
\`\`\`

Step by step:

1. A **Wi-Fi client** associates to the **hostapd** access point and begins an 802.1X exchange using the configured EAP type and inner method.
2. **hostapd** wraps the client's EAP messages into a **RADIUS Access-Request** and sends them to **FreeRADIUS** on port **1812**, authenticated with the shared secret.
3. **FreeRADIUS** validates the supplied credentials against **OpenLDAP** by performing an LDAP bind against the directory on port **3389**.
4. If the directory accepts the credentials, FreeRADIUS returns an **Access-Accept** along with the master session key (**MSK**) material the access point needs.
5. hostapd and the client run the WPA **4-way handshake** using that key, and the client reaches a fully **connected** state.

If any step fails (wrong shared secret, missing certificate, bad password, or an account that is not in the directory) FreeRADIUS returns an **Access-Reject** and the client never completes the handshake.

## How it ties together

RADIUS does not work alone. Three other pages feed into it:

- **LDAP** is the user backend that FreeRADIUS checks. Every account a client can authenticate as lives in the directory. Add, edit, or inspect those users on the [LDAP guide](/ldap/guide).
- **Certificates** provide the server certificate FreeRADIUS presents during the EAP tunnel handshake (PEAP, TTLS, TLS), and the client certificates required for **EAP-TLS**. Manage the certificate authority and issued certs on the [Certificates guide](/certificates/guide).
- **Networks** is where you create the WPA-Enterprise SSID on hostapd that points clients at this RADIUS server in the first place. Set up the access point on the [Networks guide](/networks/guide).

---

Together these four pieces (a network on hostapd, this RADIUS server, the LDAP directory, and the certificates) form a complete, realistic enterprise Wi-Fi authentication stack you can attack, defend, and demonstrate.`
	},
	certificates: {
		title: 'Certificates Guide',
		subtitle: 'The certificate authority behind EAP and WPA-Enterprise',
		doc: `
## What this page is for

WPA2-Enterprise and WPA3-Enterprise do not use a single shared passphrase. Instead, every client authenticates individually over **EAP** (the Extensible Authentication Protocol), brokered by a RADIUS server. EAP is built on TLS, and TLS is built on **X.509 certificates**. That is why this page exists: it is the certificate authority that issues and signs the certificates the rest of the enterprise stack depends on.

Two things always need a certificate:

- The **RADIUS server** must present a **server certificate** during the EAP handshake so clients can confirm they are talking to the genuine authentication server and not an impostor.
- For **EAP-TLS**, each user additionally needs a **client certificate**, which the user presents to prove their identity instead of typing a password.

A **certificate authority (CA)** is the trust anchor that signs both. A certificate is only trusted because the CA vouches for it, so nothing else on this page can be issued until the CA exists.

![The Certificates page](/guide/certificates.png)

---

## Step 1: Initialize the Certificate Authority

![The certificate authority](/guide/certs-ca.png)

Until a CA exists, the Certificate Authority panel shows a **Required** badge and a single **Initialize CA** button. Click it once.

Initializing the CA produces a self-signed **root CA**. This root is the top of the trust chain: it signs every server and client certificate you issue afterward, and its signature is what makes those certificates verifiable. The page reflects the new state right away:

- The Certificate Authority panel flips to **Active** and lists the CA's **Name**, a **Type** of \`Root CA\`, and its **Expires** date.
- The header badge at the top of the page changes from **No CA** to **CA Ready**.
- The new CA appears in the **Certificates** table below with a type of \`ca\`.

> You initialize the CA once. From that point on, the **New Certificate** panel is what you use to issue everything else. There is no separate button to issue a CA from the form; the CA is created only through the Initialize CA action.

---

## Step 2: Issue a server certificate for FreeRADIUS

![Issue a certificate](/guide/certs-new.png)

In the **New Certificate** panel, set **Type** to **Server (FreeRADIUS)** and give it a **Name**, for example \`radius-server\`. Click **Create Certificate**.

This issues an X.509 **server certificate** signed by your root CA and hands it to FreeRADIUS, the RADIUS server that runs the enterprise authentication.

Why the RADIUS server needs it: during every EAP exchange, the access point relays the conversation to RADIUS, and RADIUS presents this server certificate to the connecting client. The client checks that the certificate was signed by a CA it trusts before it sends any credentials. This is the leg of the handshake that proves the network is legitimate and protects users from a rogue access point trying to harvest credentials. **Every** enterprise EAP method (PEAP, EAP-TTLS, and EAP-TLS) relies on this server certificate, so a server certificate is the minimum needed to stand up a working WPA-Enterprise network.

---

## Step 3: Issue client certificates for EAP-TLS

Set **Type** to **Client (EAP-TLS)**. The form swaps the Name field for a **User UID** field, for example \`jdoe\`. Enter the UID and click **Create Certificate**.

This issues a **client certificate** bound to that user, again signed by your root CA.

EAP-TLS is the strongest enterprise method because authentication is **mutual** and **passwordless**. Both sides present a certificate: the RADIUS server presents its server certificate, and the client presents this per-user client certificate. RADIUS validates the client certificate against the CA, and only a holder of a CA-signed certificate gets on the network. There is no password to phish, guess, or reuse; the private key on the client is the credential. Issue one client certificate per user who needs to authenticate with EAP-TLS.

> PEAP and EAP-TTLS do not need client certificates because they authenticate the user with a username and password inside the TLS tunnel. Only EAP-TLS requires a certificate per user. Reach for client certificates specifically when you are building or training against an EAP-TLS network.

---

## The certificates list

![The certificates table](/guide/certs-table.png)

The **Certificates** table at the bottom is the inventory of everything the CA has produced. Each row shows:

- **Name** - the certificate's identifier (for client certificates this is the user UID).
- **Type** - \`ca\`, \`server\`, or \`client\`.
- **Network** - the network the certificate is associated with, or \`-\` if none.
- **Expires** - the expiry date, or \`-\` if not set.

The header also keeps a running count of how many certificates exist. Before the CA is initialized the list is empty and prompts you to initialize the CA first.

> This page lists and issues certificates. It does not expose download or delete actions, so manage the lifecycle by issuing the certificates you need; the platform wires issued certificates into the RADIUS and network configuration for you.

---

## How it all ties together

The certificate authority is the foundation the enterprise authentication stack stands on:

1. You **initialize the CA** here, creating the root that signs everything else.
2. You **issue a server certificate**, and RADIUS presents it during the EAP handshake so clients can trust the network. See the [RADIUS guide](/radius/guide) for how the authentication server uses it.
3. For EAP-TLS, you **issue client certificates**, and an enterprise network configured for EAP-TLS validates each client certificate against the CA in place of a password. See the [Networks guide](/networks/guide) for how an enterprise SSID is configured to require them.

Initialize the CA first, issue a server certificate so RADIUS can authenticate at all, then issue client certificates only if you are running EAP-TLS.
`
	},
	client: {
		title: 'Client Mode',
		subtitle: 'Run this box as a traffic-generating client',
		doc: `## Overview

In **client mode** a Tala WTE box stops broadcasting access points and instead joins another Tala WTE network as an ordinary station. It associates with the SSID, pulls a DHCP lease, gets past a captive portal if there is one, and generates realistic traffic. Use a client to populate a training network with believable activity, to give a packet capture something real to record, and to put capturable handshakes and cleartext credentials on the air for trainees.

![Client dashboard](/guide/client.png)

A box is put into client mode either at install time (\`tala-wte install-client\`) or by flipping the role from **Settings -> Instance Role**. In client mode the navigation is trimmed to **Dashboard**, **Traffic**, and **Settings** - there are no access-point pages, because a client does not broadcast.

## The client dashboard

![Connection details](/guide/client-connection.png)

![Wireless adapters](/guide/client-adapters.png)

The Dashboard is the landing page and a live status board for the client:

- **Connection** - **Online** or **Offline**, the joined SSID, and the leased IP address, summarized across the top.
- **Connection panel** - the full detail: status, SSID, interface, IP, gateway, and the captive-portal state (\`none\`, \`detected\`, \`passed\`, or \`failed\`).
- **Wireless Interfaces** - the adapter(s) this client can use to associate, with the model and chipset. If an adapter is plugged in but has no driver/firmware, it is flagged here so you know it needs a driver.
- **Traffic Generation** - whether traffic is currently running, with **Requests**, **Received**, and **Errors** counters, and a link to the traffic console.

The connection state is verified against the radio on every refresh: if the access point goes away (stops, moves, or deauthenticates the client) the dashboard flips to **Offline** rather than showing a stale connection.

## Joining a network and generating traffic

![Traffic generation status](/guide/client-traffic.png)

Click **Open traffic console** to reach the **Traffic Console**, where you import network configs, connect, and drive the traffic generators and handshake-capture cycling. See the Traffic Console guide for the full workflow.

## Switching roles

A box can be a server (access point) or a client, and you can switch with one button from **Settings -> Instance Role**. Switching installs the other role's dependencies and restarts the service into the new mode; the console reconnects on its own. See the Settings guide.

## Joining a den

A client can be driven remotely by a **den leader** so you do not have to configure each client by hand. Copy this client's **Den Agent Key** from **Settings -> Den Agent Key** and register the client on the leader's **Den** page by address and key. The leader can then push a network config, start traffic, and stop it for you. See the Den guide.`
	},
	traffic: {
		title: 'Traffic Console',
		subtitle: 'Join a network and generate realistic traffic',
		doc: `## Overview

The **Traffic Console** is where a client joins a network and generates realistic traffic against it. It keeps a library of saved networks, runs a set of traffic generators you choose, replays operator-supplied targets and credentials, and can cycle the connection to produce fresh WPA handshakes. Everything it does is real traffic on the wire, so a packet capture on the access point records exactly what a live device would send.

## Saved networks

![Saved networks](/guide/traffic-saved.png)

Export a **client config** from any access point on its network detail page, then bring it here. Drop the \`.json\` file on the upload zone or click to browse - each upload is **saved** to a library, so you can keep several networks on hand and switch between them at any time.

- **Connect** on a saved network joins it: the client associates, pulls DHCP, and auto-bypasses a captive portal when the config has one.
- The row of the network you are on is marked **connected**.
- **Disconnect** drops the link.
- **Del** removes a saved network from the library.

## Traffic generation

![Traffic generators](/guide/traffic-generators.png)

Choose which generators run, set the target scope, then **Start traffic**. The generators are:

- **Web browsing** - HTTP/HTTPS GET requests.
- **DNS lookups** - background name resolution.
- **Ping / local LAN** - ICMP echo and intra-LAN chatter.
- **Downloads / bandwidth** - periodic larger transfers.
- **Credential logins** - replays the logins you list below in cleartext (HTTP Basic and form POST) so trainees can capture and crack them.
- **Domain chatter (responder bait)** - LLMNR, NBT-NS, and mDNS name lookups, the broadcast/multicast bait that responder-style poisoning attacks feed on.

**Target scope** selects **Local targets** (the gateway and LAN hosts) and/or **Internet targets** (public hosts). **Start traffic** begins generating; **Stop** halts it while staying connected.

## Targets and credentials

![Targets and credentials](/guide/traffic-targets.png)

Make the traffic hit hosts you control instead of the built-in defaults:

- **URLs to browse**, **Domains to resolve**, and **IPs to reach** - one entry per line, fed to the web, DNS/domain, and ping generators respectively.
- **Login credentials** - URL, username, and password rows the client replays. These are sent in **cleartext on purpose**: capturing and decrypting them on the access point is the whole point of the exercise.

## Handshake capture (reconnect cycling)

![Handshake capture controls](/guide/traffic-handshake.png)

**Reconnect cycling** periodically deauthenticates and reassociates the client so students can capture a fresh WPA four-way handshake on every cycle. Set a **Frequency** (the base interval) and a **Jitter** (a random extra wait added on top so the timing is not perfectly periodic). Use the presets (30s, 1m, 2m, 5m, 15m, 1h) or enter a custom value in seconds, minutes, or hours. **Start cycling** begins; the header shows the live cycle count, and **Stop cycling** ends it while keeping the connection up.

## Live Log

![The Live Log window](/guide/traffic-livelog.png)

**Live Log** opens a draggable, resizable window streaming the full client activity log - the connection lifecycle (associating, DHCP, portal), which generators started, the reconnect cycles, and every error - so you can watch what the client is doing while you work elsewhere in the console.

## Live stats

The **Live Stats** panel tracks **Requests**, **Received** bytes, and **Errors** for the running generators, plus the last event and last error.`
	},
	den: {
		title: 'The Den',
		subtitle: 'Drive a pack of client members from one leader',
		doc: `## Overview

The **den** turns one access-point server into the leader of a pack of clients. The leader (this server) drives each registered client - a **member** - over the network: it pushes a network config, starts the chosen traffic, reports live status, and stops it again, all without anyone logging into the members. Use the den to stand up a whole room of believable clients on a training network from a single console.

![The Den](/guide/den.png)

## Agent keys

Each member authenticates the leader with an **agent key** instead of a login. On the member, open **Settings -> Den Agent Key**, then **Copy key**. The leader presents this key on every control call to the member, so the member needs no account for the leader to drive it. **Regenerate** on the member rotates the key and immediately revokes any leader still holding the old one.

## Registering a member

![Add a member](/guide/den-add.png)

On the leader's **Den** page, use **Add member**:

- **Name** - any label for the member, for example \`lab-client-1\`.
- **Address** - the member's host or \`host:port\`. The scheme \`https\` and port \`8443\` are assumed if you omit them.
- **Agent key** - pasted from the member's Settings.

Once added, the member appears in the **Members** list with a live **Reachable / unreachable** dot and its current state (idle, or connected to a network with its IP and request count). **Del** removes a member from the den.

## Deploying

![Deploy to a member](/guide/den-members.png)

For each member pick a **network** and a **profile**, then **Deploy**. The profiles bundle the full traffic configuration the leader pushes:

- **Standard traffic** - web, DNS, and ping over local and internet scope.
- **Full traffic** - every generator, including credential logins and responder bait.
- **Handshake capture** - traffic plus reconnect cycling, so the member produces a fresh WPA handshake on a schedule.

Deploy pushes the network's client config to the member, waits for it to associate, then starts the chosen traffic (and reconnect cycling for the handshake profile). The member's row then shows it connected and generating. **Stop** disconnects the member and clears its assignment.

## Teardown propagation

When you stop or delete a network on the leader, every member assigned to that network is automatically disconnected. Members never keep chasing a network that has gone away.

## Status

The Den page polls each member's live status through the leader (using the agent key), so the Members list reflects reachability, the joined SSID, IP, and request counts without you opening each member's own console.`
	},
	settings: {
		title: 'Settings',
		subtitle: 'Instance role, radio, uplink, agent key, and updates',
		doc: `## Overview

The **Settings** page is the per-box configuration: the role it runs as, the radio and uplink it uses, the den agent key (in client mode), and software updates. The same page serves both server and client mode; the panels that do not apply to the current role are simply not shown.

![Settings](/guide/settings.png)

## Instance role

![The instance role swap](/guide/settings-role.png)

Switch the box between **Server (AP)** and **Client** with one button. Switching persists the chosen role, installs the other role's dependencies, and restarts the service into the new mode; the console disconnects and reloads on its own when it is back, which can take a minute. Server mode broadcasts networks and runs the den leader; client mode joins a network and generates traffic. The persisted role wins over the install-time mode, so a box installed as one role can become the other without reinstalling.

## Den Agent Key (client mode)

![The den agent key](/guide/settings-agentkey.png)

In client mode the **Den Agent Key** is the control token a den leader uses to drive this client. **Copy key** puts it on your clipboard to paste into a leader's Den page; **Regenerate** issues a new key and revokes any leader still using the old one. See the Den guide.

## Radio & Network

![Radio and network](/guide/settings-radio.png)

- **Regulatory Domain** - the country hostapd advertises, applied with \`iw reg set\`. It decides which channels are legal and whether 5 GHz / 6 GHz AP mode is allowed; the world domain blocks 5 GHz beaconing, so this must match where the box actually operates.
- **Uplink Interface** - the interface connected to the internet, used for NAT passthrough on networks that allow it (for example \`eth0\`).
- **Default Network Subnet** - the default LAN/CIDR handed to clients that join a network. The gateway is \`.1\` and DHCP serves \`.10\` through \`.250\`. Each network can override this when it is created.

Click **Save Changes** to apply the regulatory domain, uplink interface, and default subnet.

## Wireless Interfaces and Services

The right column lists the box's **Wireless Interfaces** with their model, chipset, capabilities, and MAC, and the backing **Services** (PocketBase, FreeRADIUS, OpenLDAP, and the portal server) with their ports, so you can confirm the stack is up at a glance.

## Software Updates

Shows the **Installed** and **Latest release** versions. On a released build, updating downloads the verified binary, replaces the running service, and restarts it; the console reconnects automatically. Development builds disable in-place updates - install a released binary to enable them.`
	}
};
