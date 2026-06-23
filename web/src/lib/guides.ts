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
		subtitle: 'Broadcast realistic access points across every Wi-Fi security protocol',
		doc: `## What this page is for

The **Networks** page is where you stand up the Wi-Fi access points your students attack, capture, and analyze. One Tala WTE box can broadcast a believable corporate SSID, an open coffee-shop hotspot with a captive portal, and a WPA3 guest network - each a real, on-the-air AP a phone or laptop will actually join.

![The Networks page](/guide/networks.png)

The table lists every network with its SSID, protocol, band, channel, the adapter it claimed, and a live status dot (stopped / running / error). Filter by SSID or protocol, sort any column, and use the per-row actions: **Start**, **Stop**, **Details**, and **Del**.

## Creating a network

Click **+ New Network**. The form has three sections: Network Profile, Hardware, and Topology.

![The new-network form](/guide/networks-new.png)

### Security Protocol - pick the lesson

This is the most important choice; it decides what students practice.

- **Open (No Auth)** - no encryption. Pick this for open-hotspot and **captive-portal** labs (enabling Open reveals the Captive Portal Sandbox option). Example: a coffee-shop SSID that harvests emails.
- **WEP (Insecure - Legacy)** - 40/104-bit. For demonstrating why WEP is dead. Key is 5 or 13 ASCII characters (or 10/26 hex); anything else is fitted to a valid key automatically and the effective key is shown.
- **WPA (TKIP - Legacy)** / **WPA2-Personal (AES)** - passphrase, 8-63 characters. WPA2-Personal is the everyday choice for handshake-capture labs. Example: passphrase \`Summer2026!\` for a four-way-handshake exercise.
- **WPA2 + WPS** - for teaching the WPS attack surface. Every WPS network here ships a recoverable 8-digit AP PIN, so the **online PIN brute force** (reaver, bully) always has a target. By default the AP uses strong WPS nonces and resists **Pixie Dust**, which is how a modern router behaves; to also make it Pixie-vulnerable, turn on **Pixie-Dust Downgrade** in Topology (below).
- **WPA3-Personal (SAE)** - modern, PMF required. **WPA3-Transition (SAE+PSK)** runs SAE and WPA2-PSK side by side for mixed fleets.
- **WPA2-Enterprise (802.1X) / WPA3-Enterprise (Suite-B)** - the corporate lesson: each user logs in with a directory identity, validated by RADIUS against the LDAP directory. These need a CA, a server certificate, LDAP users, and FreeRADIUS running (see the [Certificates](/certificates/guide), [RADIUS](/radius/guide), and [LDAP](/ldap/guide) guides). You provide an **EAP Identity (directory username)** and **EAP Password** (a real directory user); the pack hands these to deployed members so they authenticate for real.

### Hardware

- **Frequency Band** (2.4 GHz / 5 GHz / **6 GHz (Wi-Fi 6E)**) and **Channel** - bands the adapter cannot host as an AP are disabled automatically, and the channel list updates per band (DFS channels are marked).
- **Wireless Interface** - the adapter that will broadcast. Adapters already claimed by a running network are shown as in-use, and any capability limits (e.g. "No WPA3-SAE (legacy chipset)") are flagged here.

### Topology

- **Internet Passthrough** (on by default) - NATs client traffic to your uplink so the network has real internet. Turn it off for an isolated, local-only range.
- **Network Subnet** - the client subnet (default \`10.0.0.0/24\`); the gateway is \`.1\` and DHCP serves \`.10\`-\`.250\`.
- **Client Isolation** - stops clients from talking to each other (hotspot-style).
- **Hidden Network** - does not beacon the SSID; clients must type the name. Good for "find the hidden SSID" exercises - it is obscurity, not security.
- **Pixie-Dust Downgrade** (WPS networks only, off by default) - decides whether the WPS network can be Pixie'd. **Off**, the AP behaves like a modern router: its registrar nonces are unpredictable, so **Pixie Dust fails** and the only way in is the slower **online PIN brute force** (reaver/bully). **On**, the AP's WPS secret nonces become predictable, so **pixiewps recovers the PIN offline in seconds** - the old-chipset flaw. Turn it on to teach Pixie Dust; leave it off to show why a patched AP defeats it and forces the online attack instead.

![A WPS network with the Pixie-Dust Downgrade toggle](/guide/networks-wps.png)

### Captive Portal Sandbox (Open networks only)

Enable it, choose a **Portal Module** (any template from Captive Portals), and a live preview appears. If the chosen portal validates credentials (a hotel, login, voucher, or membership portal), a **Credential set** selector appears, labeled with the auth type (e.g. "Credential set (hotel validation)"), so submissions are checked against a real list - see the [Portal Guide](/portals/guide). Its first option is **No set - capture only** (record what users enter without validating); if no matching set exists yet, the form shows a **Generate a credential set** link instead. You can also turn on **Require Login (Directory / LDAP)** to validate portal logins against the directory like a corporate hotspot.

## Starting and stopping

![The enterprise preflight gate](/guide/networks-preflight.png)

Click **Start**. For a personal/open network it comes up immediately. Tala WTE **automatically claims a free adapter** if the one you saved with is gone (it even falls back to a band the new adapter supports), so a replugged or swapped USB card just works. When the saved adapter is missing, starting from the **Details** page pops a confirm dialog proposing a specific free adapter (and a band change if needed) for you to approve, rather than picking silently.

For a **WPA-Enterprise** network, a preflight gate checks the CA, server cert, LDAP users, and FreeRADIUS. If anything is missing the button becomes **Auto-provision & Start** - one click bootstraps the whole enterprise stack, then starts the network.

The **detail page** shows live status, connected clients (MAC, IP, signal), and a streaming log, plus **Export client config** - a \`.json\` profile you import on a Tala WTE client (or hand to the Pack) so it can join this exact network.

A live **Protocol Guide** side panel on the new-network and detail pages explains the protocol you have selected (its cipher, what it teaches, and gotchas), updating as you change the Security Protocol.

![The Protocol Guide panel](/guide/networks-protocol.png)

## Tips
- Start with **WPA2-Personal** for handshake labs and **Open + a captive portal** for credential-harvesting labs - they are the two highest-value exercises.
- Enterprise looks like a lot of setup; **Auto-provision & Start** does it for you on the first run.
- Turn **Internet Passthrough** off when you want a sealed range with no real internet egress.`
	},
	captures: {
		title: 'Packet Captures Guide',
		subtitle: 'Record live wireless or network traffic and analyze it in the browser',
		doc: `## What this page is for

**Packet Captures** records real traffic off a running network and analyzes it for you - protocol mix, top talkers, DNS and HTTP requests, the HTTPS sites contacted, and **cleartext credentials pulled straight off the wire**. It is the payoff page: students see exactly what a sniffer sees.

![The Packet Captures page](/guide/captures.png)

## Starting a capture

![Start a new capture](/guide/captures-start.png)

- **Network** - which running network to record.
- **Layer**:
  - **Network (IP layer - tshark on AP interface)** - runs tshark on the AP's interface, inside that network's sandbox. This is the everyday choice: it sees the IP traffic of every client on that network (DNS, HTTP, logins).
  - **Wireless (802.11 - monitor mode interface)** - a monitor-mode capture of raw 802.11 frames (beacons, handshakes, management frames).
- **Interface** - the adapter to capture on.
- **BPF Filter (optional)** - narrow the capture with Berkeley Packet Filter syntax. Use the preset chips for the common ones: **HTTP** (\`tcp port 80\`), **TLS / HTTPS** (\`tcp port 443\`), **DNS** (\`udp port 53\`), **DHCP**, **ARP**, **ICMP**, or **Clear**. Example: capture only logins with the HTTP preset.

Then **Start Capture**. The session appears in the list with a live packet count.

> **Seeing 0 packets?** That is not a bug - a network-layer capture only records traffic that actually crosses the AP. You need a client doing something. Connect a client (or deploy a Pack member with traffic generation) on that network, then watch the count climb. An idle network with no clients captures nothing.

## Reading a capture

Each session in the list has row actions: **Stop** (while it is running), **View**, **Download** (the pcap), and **Del**. Open a stopped capture with **View**. The header shows packets, duration, size, and protocol count, with two tabs.

![The PCAP analysis view](/guide/capture-analysis.png)

**Analysis** turns the pcap into a readable story:
- **Cleartext credentials** (in red) - a Type / Host / Captured table of logins recovered from the traffic, where Type is **HTTP Basic** or **HTTP form post** (e.g. \`username=jdoe  password=Hunter2!\`). This is the lesson: anything not encrypted is visible.
- **Protocol mix** - a bar chart of what was on the wire (TCP, DNS, HTTP, ARP, DHCP...).
- **Top talkers** - the busiest conversations by packet count.
- **HTTP requests** (method / host / URI), **TLS server names (SNI)** - the HTTPS sites contacted even though the payload is encrypted - **DNS queries**, and **HTTP user agents**.

**Packets** is a Wireshark-style list (No., Time, Source, Destination, Protocol, Len, Info). Type a display filter (e.g. \`http\`, \`dns\`, \`ip.addr==10.0.0.50\`) and **Apply**; click any packet for the full decoded protocol tree.

**Download pcap** saves the file to open in Wireshark.

## Tips
- Pair a capture with a Pack deploy: start the capture, deploy a member that browses + replays a login, and the credentials show up in the Analysis tab within seconds.
- Use the **HTTP** or **DNS** presets to keep captures small and focused for a specific lesson.
- If the analysis says tshark is unavailable, only summary counts appear - reinstall completes the toolset.`
	},
	portals: {
		title: 'Portal Guide',
		subtitle: 'Captive portals that look real, validate logins, and harvest what users enter',
		doc: `## What this page is for

A **captive portal** is the splash page a device sees after joining an open Wi-Fi network, before it can reach the internet. Tala WTE ships 36 built-in portals that mirror real venues (hotels, airports, coffee shops, corporate guest pages, ISPs, gyms) and lets you build your own. Every value a user enters is harvested to **Captured Data**, and portals that should check logins do - exactly like the real thing.

![The portal library](/guide/gallery.png)

## The library

Each card has actions for that portal. Built-in cards show **Customize**, **Preview**, **Clone**, and **Delete**; your own cards show **Edit** (instead of Customize), plus **Preview**, **Clone**, and **Delete**. (Clone is hidden for uploaded \`.zip\` bundle portals.) You assign a portal to a network on the Networks form, not from the card.

- **Built-in** templates ship with the app and each already has the right auth type. You cannot edit a built-in in place (Customize clones it first, so the originals stay pristine); **Restore Templates** at the top re-seeds any you delete and resets edited ones.
- **Custom** templates are ones you create, **Clone from URL** (scrape a real sign-in page; private/internal addresses are blocked), or **Upload Template** (an \`.html\` file or a \`.zip\` bundle with assets).
- **Search**, filter by **All / Built-in / Custom**, **Sort**, and switch **Cards / List** (your choice is remembered). In card view, hovering shows a live mobile preview.

## Auth types - how a portal behaves

Every portal conforms to one **auth type**. This decides what fields it shows, whether it validates, and what gets captured. Three just collect; five validate against a credential set.

| Auth type | Collects | Validates? | Use it for |
|---|---|---|---|
| **Click-through** | accept terms | no | "Connect" splash, terms acceptance |
| **Email capture** | email | no | marketing gate (coffee shop, retail) |
| **Information form** | first name, last name, email, phone, company | no | guest registration, lead capture |
| **Username & password** | username + password | yes | corporate / AD login pages |
| **Email & password** | email + password | yes | ISP / webmail-style sign-in |
| **Hotel (room + last name)** | last name + room number | yes | hotel / cruise Wi-Fi |
| **Voucher / access code** | a code | yes | conference, event, transit ticket |
| **Membership (ID + PIN)** | member ID + PIN | yes | gym / loyalty hotspot |

A custom portal's auth type is set in the editor; built-ins are pre-typed and read-only.

## Credential Sets - make validation real

A **credential set** is a list of valid logins for one auth type. Assign a set to a network's portal and submissions are checked against it: a match grants access, a wrong entry is rejected and re-prompted - just like a real hotel front desk or corporate login.

![Generate and manage credential sets](/guide/portals-credentials.png)

On this page, the **Credential Sets** panel lets you:
- **Generate set**: pick the auth type, a count (prefilled at 25), and a name like "Harbor Hotel Guests", and Tala WTE creates believable, validatable entries (e.g. last name + room number).
- **View** a set to see every entry, and **Del** to remove it.

You assign a set to a network on the **Networks** form (the **Credential set** selector appears when the chosen portal validates).

**You usually do not have to do any of this.** When a validating portal starts with no set assigned, the leader **auto-generates one** and a deployed pack member automatically receives a working credential and passes the portal - so a hotel SSID "just works" out of the box, and Captured Data fills with believable logins on its own. Credential field names are matched flexibly, so one hotel set works whether the form calls the field \`room_number\` or \`stateroom\`.

## Creating and editing

![Four ways to add a portal](/guide/portals-actions.png)

**Clone** a built-in for the fastest start, **+ New Portal** for the **New Captive Portal** editor (with a **Start From Template** picker and an **Insert Starter HTML** button), **Clone from URL** to import a real page, or **Upload Template**. The editor shows HTML on the left and a live desktop/mobile preview on the right, lists the fields the portal will capture, and lets you set the **auth type**. On save and on serve, Tala WTE wires the first form to the capture endpoint automatically - you do not hand-edit form actions.

![The portal editor](/guide/editor.png)

## Capturing what users enter

![Captured data](/guide/captured.png)

Every submission lands in **Captured Data** in a table of Network, **Captured** (timestamp), Username, Password (highlighted), Result, Source, MAC, and IP. The **Source** tag reads **pack member** for simulated pack traffic vs a real **target**, so you can tell training noise from a live capture.

## Assigning a portal to a network

![Assign a portal in the network form](/guide/portals-assign.png)

1. Create or edit a network and set **Security Protocol** to **Open**.
2. Enable the **Captive Portal Sandbox** and choose your portal.
3. For a validating portal, pick a credential set (or leave it - one is auto-generated).
4. Start the network. Connecting clients are intercepted until they satisfy the portal.

## Tips
- Match the venue: the closer the portal looks to what a client expects, the more convincing the exercise.
- For credentialed lessons, just start the network - the auto-generated set + a Pack deploy populate Captured Data for you.
- Use **View** on a credential set to read out a valid login if you want to test the portal by hand.`
	},
	ldap: {
		title: 'LDAP Directory Guide',
		subtitle: 'The embedded directory that backs enterprise Wi-Fi and credentialed portals',
		doc: `## What this page is for

Tala WTE runs a real **OpenLDAP** directory (base \`dc=tala,dc=wte\`) that acts as your fake company. It is the user store RADIUS checks for WPA-Enterprise logins, and it can back captive-portal "Require Login" validation. Populate it once and your enterprise and portal labs have believable people to authenticate.

![The LDAP Directory page](/guide/ldap.png)

A stat strip across the top shows the **Base DN** (\`dc=tala,dc=wte\`), the **Bind DN** (\`cn=admin,dc=tala,dc=wte\`), the LDAP **Port** (\`3389\`), and the current user count. Below it, the directory is organized into three tabs - **Users**, **Groups**, and **Test Auth** - with the provisioning controls in a panel above them.

## Provisioning a company

![Directory provisioning](/guide/ldap-provision.png)

The fastest start is **Generate Random Company** - it builds a believable org (a name like "Vanguard Industries", a matching domain, 10-20 users across real departments with real job titles) and a realistic password mix (some weak, some semi-personal, some strong) so cracking exercises feel real. You can also **Reset to Default (ACME Corp)** or go **Custom** (your own company name, email domain, a user count up to 50, and an **All Strong Random Passwords** toggle). Each provision wipes and rebuilds the directory and shows you the generated users and passwords.

## Users

![The Users tab](/guide/ldap-users.png)

The **Users** tab lists every account with its username (\`uid\`), full name (\`cn\`), job title, department, email, and password; filter the list or sort by UID, name, title, or department. Plaintext passwords can be shown/copied for setting up test clients; hashed ones read "(hashed)". Add a user with the inline form and the **Add User** button (UID, full name, last name, email, password); per row, **Set pw** changes a user's password (handy to make a weak account strong, or to set a known credential for test clients) and **Del** removes them.

## Groups

![The Groups tab](/guide/ldap-groups.png)

The **Groups** tab shows realistic domain groups - **Domain Users**, **Domain Admins**, per-department groups (Engineering, Sales, IT, Finance...), operator groups (Help Desk, Backup Operators), access groups (VPN Users, Remote Desktop Users), and the **wifi-users** / **wifi-admins** groups; filter or sort by group name or member count. Create a group with the inline field, **Del** to remove one, and manage membership inline: each member shows as a chip you click to remove, and an **add uid** field with **Add** puts a user in the group. Group membership is what you reference when scoping access in a lesson.

## Test Auth

![The Test Auth tab](/guide/ldap-testauth.png)

Before you stand up an enterprise network, use **Test Auth**: enter a user's UID and password and Tala WTE performs a real LDAP bind, returning success (with the DN) or failure. This is the quickest way to confirm a credential works end to end.

## Tips
- Provision the directory **before** starting a WPA-Enterprise network or a "Require Login" portal - both authenticate against it.
- Leave the default mixed-password setting on for realistic cracking labs; flip to all-strong when you want auth to always succeed.
- The EAP Identity/Password you put on an enterprise network must be a real user here - verify it on Test Auth first.`
	},
	radius: {
		title: 'RADIUS Guide',
		subtitle: 'The 802.1X authentication server behind WPA-Enterprise networks',
		doc: `## What this page is for

**FreeRADIUS** is the gatekeeper for WPA2/WPA3-Enterprise networks. When a client does 802.1X, the access point asks RADIUS, and RADIUS checks the user against the LDAP directory. This page configures how that handshake works.

![The RADIUS page](/guide/radius.png)

## EAP configuration

![EAP configuration](/guide/radius-eap.png)

- **Default EAP Type** - this actually reconfigures the running server (it writes \`default_eap_type\` into the FreeRADIUS eap module):
  - **EAP-PEAP (Recommended)** - username/password inside a TLS tunnel; the common enterprise setup.
  - **EAP-TLS (Certificate Auth)** - certificate-based, no password (issue client certs on the Certificates page).
  - **EAP-TTLS** - a tunneled method that also allows a PAP inner.
- **Inner Authentication** (for tunneled EAP) - **MSCHAPv2** (the default, used by PEAP) or **PAP** (available for TTLS). The server validates both against the directory.
- **Shared Secret** - the secret between the AP and RADIUS; leave blank to use a generated one.

Click **Save Configuration** to apply the EAP type and restart FreeRADIUS. It is also applied automatically by **Auto-provision & Start** on an enterprise network. Pick PEAP/MSCHAPv2 for a standard "corporate Wi-Fi login" lesson; pick EAP-TLS to teach certificate-based auth.

## The authentication chain

![The authentication chain](/guide/radius-chain.png)

The page draws the full path so students can see where their login travels:

\`\`\`
Wi-Fi client -> EAP -> hostapd (AP) -> RADIUS :1812 -> LDAP :3389 -> Access-Accept -> 4-way handshake -> connected
\`\`\`

From here you can jump straight to **Manage LDAP Users** and **Manage Certs**, the other two pieces of the enterprise stack.

## Tips
- You rarely need to touch this for a basic enterprise lab - the defaults (PEAP/MSCHAPv2) work, and **Auto-provision & Start** on the network sets RADIUS up for you.
- Use EAP-TLS only when the lesson is about client certificates; it needs a client cert per user.`
	},
	certificates: {
		title: 'Certificates Guide',
		subtitle: 'The certificate authority behind EAP and WPA-Enterprise',
		doc: `## What this page is for

WPA-Enterprise needs TLS, and TLS needs certificates. This page runs a small **certificate authority**: initialize a root CA, then issue the server certificate FreeRADIUS presents and (for EAP-TLS) per-user client certificates.

![The Certificates page](/guide/certificates.png)

## Initialize the CA

![The certificate authority](/guide/certs-ca.png)

If no CA exists, the page shows a warning and an **Initialize CA** button. Click it to create a 10-year root CA (\`O=Tala WTE, CN=Tala WTE CA\`). Once active, the panel reads "The CA is initialized" and you can issue certificates.

## Issue a certificate

![Issue a certificate](/guide/certs-new.png)

- **Server (FreeRADIUS)** - the cert RADIUS presents during EAP. Name it e.g. \`radius-server\`. Required for any enterprise network.
- **Client (EAP-TLS)** - a per-user certificate for certificate-based auth. Enter the user's UID (e.g. \`jdoe\`); it is signed by your CA with the CN \`jdoe-client\`.

![The certificates table](/guide/certs-table.png)

The table lists every certificate with its name, type (CA / server / client), associated network, and expiry. Tala WTE keeps this in sync with what is actually on disk, so the page always reflects reality.

## Tips
- The order is **CA -> server cert -> (optional) client certs**. For a password-based PEAP lab you only need the CA and a server cert.
- You usually do not have to do any of this by hand - **Auto-provision & Start** on a WPA-Enterprise network creates the CA and server cert automatically.
- Issue client certificates only when the lesson is EAP-TLS.`
	},
	client: {
		title: 'Client Mode',
		subtitle: 'Run this box as a Wi-Fi client that joins networks and generates traffic',
		doc: `## What this page is for

A Tala WTE box can be flipped into **Client mode** to become a believable Wi-Fi client - it joins a target network, gets past a captive portal, and generates realistic traffic so there is something to capture and analyze. It is the other half of a lab: one box broadcasts, another behaves like a user.

![Client dashboard](/guide/client.png)

Switch roles on the [Settings](/settings/guide) page ("Switch to Client mode"). In client mode the box stops being an access point and shows the **Dashboard** (subtitle "Traffic generation agent status"). This page is read-only status; you actually connect and drive traffic from the **Traffic Console** (the **Open traffic console** link, and see the [Traffic Console guide](/traffic/guide)).

## Reading the dashboard

![Connection details](/guide/client-connection.png)

A stat strip across the top shows **Connection** (Online / Offline), the **Network** you are on, your **IP Address**, and the count of **Wireless Adapters**. A **Traffic Generation** panel shows whether it is **Generating**, plus live **Requests**, **Received**, and **Errors**.

![Wireless adapters](/guide/client-adapters.png)

If no adapter is present, the page reads "No wireless adapter detected. Plug in a USB adapter to join a network." Adapters that are present but lack driver support raise a warning so you install the driver before connecting.

## Connecting and generating traffic

![Traffic generation status](/guide/client-traffic.png)

Open the **Traffic Console** to import a network profile (the \`.json\` you exported from a network's detail page), **Connect**, and run the generators - web, DNS, ping, downloads, credential logins, and responder bait, against local and/or internet targets. The Dashboard then reflects the live connection and traffic stats.

## Tips
- For a controlled lab, prefer driving clients from the **Pack** (one leader, many members) instead of flipping individual boxes - same engine, central control.
- Client mode is ideal for a single demo box you connect to a target AP by hand.`
	},
	traffic: {
		title: 'Traffic Console',
		subtitle: 'Join a network and generate realistic, capturable traffic',
		doc: `## What this page is for

The **Traffic Console** makes a client behave like a real device: it joins a saved network, gets past a captive portal, and runs the traffic generators you choose - so a packet capture on the AP records exactly what a live user would send, including cleartext logins to crack.

## Saved networks

![Saved networks](/guide/traffic-saved.png)

Export a **client config** from any network's detail page, then drop the \`.json\` here (or click to browse). Each upload is saved to a library so you can switch between networks. **Connect** joins one (associate, DHCP, and auto-bypass a captive portal if it has one); **Disconnect** drops the link; **Del** removes a saved network.

## Traffic generators

![Traffic generators](/guide/traffic-generators.png)

Choose which generators run, set the scope, then **Start traffic**:

- **Web browsing** - HTTP/HTTPS GETs to your URLs and (with Internet scope) safe public sites.
- **DNS lookups** - background name resolution.
- **Ping / local LAN** - ICMP echo and intra-LAN chatter.
- **Downloads / bandwidth** - periodic larger transfers.
- **Credential logins** - replays the logins you list below in **cleartext** (HTTP Basic and form POST) so trainees can capture and crack them.
- **Domain chatter (responder bait)** - LLMNR, NBT-NS, and mDNS lookups for names like \`wpad\`, \`fileserver\`, and \`intranet\` - the exact broadcast/multicast bait that Responder-style poisoning attacks feed on. Only worth enabling when a Responder/Inveigh-style listener is running on the wireless side to catch it.

**When to enable which:** Web/DNS/Ping keep a network looking alive for any capture; **Downloads** add bulk for bandwidth/throughput demos; **Credential logins** are the one to turn on for capture-and-crack labs (they emit cleartext logins); **Domain chatter** only matters with a poisoning listener present.

**Target scope** - **Local targets** (the gateway and LAN hosts) and/or **Internet targets** (public hosts). **Live Stats** tracks **Requests**, **Received**, and **Errors** as it runs.

## Targets and credentials

![Targets and credentials](/guide/traffic-targets.png)

Point the traffic at hosts you control instead of the defaults:
- **Apply a traffic dataset** - pick a saved dataset to fill the target fields in one step (the same reusable lists managed on the Pack page), then tweak them.
- **URLs to browse**, **Domains to resolve**, **IPs to reach** - one per line, fed to the web, DNS/domain, and ping generators.
- **Login credentials** - URL, username, and password rows the client replays. These go out in **cleartext on purpose** - capturing and decrypting them is the whole point.

## Handshake capture (reconnect cycling)

![Handshake capture controls](/guide/traffic-handshake.png)

**Reconnect cycling** periodically deauthenticates and reassociates so students can capture a fresh WPA four-way handshake every cycle. Set a **Frequency** (30s up to 1h, or Custom) and a **Jitter** (a random extra wait so timing is not robotic). **Start cycling** begins; the header shows the live cycle count; **Update cycling** changes the timing without stopping; **Stop cycling** ends it while staying connected.

## Live Log

![The Live Log window](/guide/traffic-livelog.png)

**Live Log** opens a draggable, resizable window streaming the full client activity log - associating, DHCP, portal, which generators started, each reconnect cycle, and every error - so you can watch the client while you work elsewhere.

## Tips
- For a credential-capture lab: add a login under **Login credentials**, enable **Credential logins**, Start traffic, then capture HTTP on the AP - the username/password appear in the capture's Analysis tab.
- Use **reconnect cycling** at 30s-1m for repeated handshake captures.
- Datasets keep target lists reusable across the Console and Pack deploys.`
	},
	pack: {
		title: 'The Pack',
		subtitle: 'Drive a whole pack of client members from one leader',
		doc: `## What this page is for

The **Pack** turns one Tala WTE box into the leader of a pack of clients - the **members**. From one console you push a network config to each member, start the traffic you want, get past captive portals, and watch live status, all without logging into any member. It is how you stand up a believable room full of clients on a training network from a single screen.

![The Pack](/guide/pack.png)

## Agent keys

Each member authenticates the leader with an **agent key** instead of a login. On the member, open **Settings -> Pack Agent Key** and **Copy key**; the leader presents it on every call. **Regenerate** on the member rotates it and instantly cuts off any leader holding the old one.

## Registering a member

![Add a member](/guide/pack-add.png)

Use **Add member**: a **Name** (e.g. \`lab-client-1\`), the member's **Address** (\`host\` or \`host:port\`; \`https\` and \`8443\` are assumed), and the **Agent key** from its Settings. Or use **Discovered on LAN -> Scan**: the leader finds other Tala WTE instances over mDNS (handy for fresh installs or changed DHCP addresses); click **Use** to fill the form, then paste the key.

## Deploying

![Deploy to a member](/guide/pack-members.png)

For each member pick a **network**, a **profile**, and optionally a **traffic dataset**, then **Deploy**:

- **Standard traffic** - web, DNS, and ping over local + internet. The general-purpose choice to keep a network alive.
- **Full traffic** - every generator, including credential logins and responder bait. Pick this when a capture (or a Responder-style listener) is running to catch the cleartext logins + poisoning bait; otherwise Standard is plenty.
- **Handshake capture** - **Standard traffic plus reconnect cycling** (it does not add credential logins or downloads), so the member produces a fresh WPA handshake on a schedule.

The **traffic dataset** sets where that traffic goes; leave it on **Default targets** or pick a saved dataset. Deploy pushes the network's client config, waits for the member to associate, then starts the traffic. Per member you also get **Stop** (disconnect + stop traffic) and **Del** (remove the member) alongside **Deploy**.

**If the network has a captive portal, the member passes it automatically.** The leader sends the member a valid credential drawn from the network's credential set (auto-generated if you did not assign one), and the member fills the real form - a hotel room + last name, a voucher, an AD login - so the portal grants access and harvests a believable login, tagged **pack member** in Captured Data. No per-member setup.

## Member status at a glance

Each card shows a status badge: **checking** (before the first status comes back), **connected** (green), **idle**, **no adapter** (yellow), **unreachable** (red), and **radio wedged** (red) - the member is reachable but its wireless driver stopped responding and needs the adapter power-cycled or replugged. A member that is connected but not assigned to one of your networks also shows an **in use by another pack leader** note (another leader is driving it). The card also lists the member's capability limits and version. The management view stays up even when a radio wedges, so you always see the real state.

## Traffic Datasets

The **Traffic Datasets** panel manages reusable target lists, with full **Add / Edit / Update / Del** and a per-dataset **Targets** summary (N URLs, N domains, N IPs) and a built-in/custom **Type** badge. Built-in sets cover connectivity checks, general browsing, local intranet, and DNS chatter; add your own with a name, description, and lists of URLs, domains, and IPs (one per line). A dataset's lists feed the web, DNS, and ping generators of any member you deploy with it - the same datasets the [Traffic Console](/traffic/guide) uses.

## Teardown propagation

Stop or delete a network on the leader and every member assigned to it is automatically disconnected - members never chase a network that has gone away. **Update all members** pushes the latest release to the whole pack.

## Tips
- The fastest believable lab: register two members, deploy both to an open + portal network with **Standard traffic**, and watch Captured Data and captures fill on their own.
- A **radio wedged** badge means a hardware reset (power-cycle/replug the adapter), not a software error.
- Use **Handshake capture** profile + a packet capture to mass-produce WPA handshakes.`
	},
	settings: {
		title: 'Settings',
		subtitle: 'Instance role, radio/regulatory domain, uplink, agent key, and updates',
		doc: `## What this page is for

**Settings** is the box-level configuration: whether this instance is an access point or a client, the regulatory domain that governs its radio, the internet uplink, the pack agent key, and software updates.

![Settings](/guide/settings.png)

## Instance role

![The instance role swap](/guide/settings-role.png)

Switch between **Server (AP)** - broadcasts networks - and **Client** - joins a network and generates traffic. Switching installs the other role's dependencies and restarts the service; the console reconnects on its own (give it a minute).

## Pack Agent Key (client mode)

![The pack agent key](/guide/settings-agentkey.png)

In client mode this shows the **agent key** a pack leader needs to drive this client. **Copy key** to paste into the leader's Add Member form; **Regenerate** rotates it (any leader on the old key loses access until you re-add the client).

## Radio and network

![Radio and network](/guide/settings-radio.png)

- **Regulatory Domain** - a dropdown of countries that hostapd advertises (applied with \`iw reg set\`). This decides which channels are legal and whether 5/6 GHz AP mode is allowed. Set it to where the box actually operates - the world domain blocks 5 GHz beaconing, so a wrong value can stop 5 GHz networks from coming up.
- **Uplink Interface (Internet)** - the interface used for NAT passthrough. A configured value that no longer exists is ignored and the real uplink is auto-detected, so a hardware change will not silently break client internet.
- **Default Network Subnet** - the CIDR new networks hand out by default (gateway \`.1\`, DHCP \`.10\`-\`.250\`); each network can override it.

Click **Save Changes** to apply.

## Wireless interfaces, services, and updates

The right column lists detected **Wireless Interfaces** (free and in-use), with a **Heal** button on any **Unsupported** adapter (runs a USB reset to recover a wedged adapter), and the running **Services** (PocketBase, FreeRADIUS, OpenLDAP, portal server). **Software Updates** shows the installed and latest versions, an **Update available** badge and a **Release notes** link when one is out, and a one-click **Update** that downloads the verified binary, replaces the service, and restarts (the console reconnects automatically). Development builds disable in-place updates. An **About & License** panel shows the Tala WTE version and copyright, with a **View Full License** button.

## Tips
- Set the **Regulatory Domain** first - it is the usual reason a 5 GHz network will not broadcast.
- If client internet breaks after swapping NICs, you no longer need to fix the uplink by hand - it auto-detects.
- Use **Heal** on an unsupported/wedged adapter before assuming the hardware is dead.`
	}
};
