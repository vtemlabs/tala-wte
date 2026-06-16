<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, paid training, paid CTF,
  or any for-profit use requires a license from VTEM Labs. See the LICENSE file.
-->
<script lang="ts">
  import GuidePage from '$lib/components/GuidePage.svelte';

  const DOC = `
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
`;
</script>

<GuidePage
  title="Packet Captures Guide"
  subtitle="Passive wireless and network-layer capture and analysis"
  backHref="/captures"
  backLabel="Back to Captures"
  crumb="Captures"
  doc={DOC}
/>
