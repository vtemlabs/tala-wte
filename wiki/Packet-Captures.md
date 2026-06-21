Packet Captures records real traffic off a running network and analyzes it for you: protocol mix, top talkers, DNS and HTTP requests, the HTTPS sites contacted, and cleartext credentials pulled straight off the wire. It is the payoff page, where students see exactly what a sniffer sees.

Captures run against a network you stood up on the [[Networks]] page, and pair naturally with a captive-portal or credential lab (see [[Captive-Portals]] and [[Credential-Sets]]).

![The Packet Captures page](images/captures.png)

## Starting a capture

![Start a new capture](images/captures-start.png)

In the "Start New Capture" panel:

- Network - which running network to record (each option shows the SSID and its status).
- Layer:
  - Network (IP layer - tshark on AP interface) - runs tshark on the AP's interface, inside that network's sandbox. This is the everyday choice; it sees the IP traffic of every client on that network (DNS, HTTP, logins).
  - Wireless (802.11 - monitor mode interface) - a monitor-mode capture of raw 802.11 frames (beacons, handshakes, management frames). Pick this for handshake/frame-level work.
- Interface - the adapter to capture on.
- BPF Filter (optional) - narrow the capture with Berkeley Packet Filter syntax. Preset chips fill the common ones for you:
  - HTTP (`tcp port 80`)
  - TLS / HTTPS (`tcp port 443`)
  - DNS (`udp port 53`)
  - DHCP (`udp port 67 or udp port 68`)
  - ARP (`arp`)
  - ICMP (`icmp`)
  - Clear (empties the filter)

Click "Start Capture". The session appears in the Capture Sessions table with a live packet count.

## "0 packets" means no client traffic

Seeing 0 packets is not a bug. A network-layer capture only records traffic that actually crosses the AP, so it needs a client doing something. Connect a client, or deploy a pack member with traffic generation, on that network and the count climbs. An idle network with no clients captures nothing.

## The Capture Sessions table

Each session lists SSID, Layer, Interface, Packets, and Status (with a colored dot). Sort any column. Row actions depend on state:

- While running: Stop.
- When stopped: View (open the analysis), Download (the pcap), and Del. Deleting the record preserves the pcap file on disk.

## Reading a capture

Open a stopped capture with View. The header shows the layer, interface, and any filter, with a "Download pcap" button. A stat strip shows Packets, Duration, Size, and Protocol count. Two tabs follow: Analysis and Packets.

### Analysis

![The capture analysis view](images/capture-analysis.png)

The Analysis tab turns the pcap into a readable story. Sections appear only when there is data for them:

- Cleartext credentials (shown in red) - a Type / Host / Captured table of logins recovered from the traffic. Type is "HTTP Basic" or "HTTP form post". This is the lesson: anything not encrypted is visible.
- Protocol mix - a bar chart of what was on the wire (TCP, DNS, HTTP, ARP, DHCP, and so on), top protocols by packet count.
- Top talkers - the busiest conversations by packet count (Endpoint A, Endpoint B, Packets).
- HTTP requests - Method, Host, and URI for requests seen.
- TLS server names (SNI) - the HTTPS sites contacted, with a count each, even though the payload is encrypted.
- DNS queries - resolved names with a count each.
- HTTP user agents - the User-Agent strings seen.

If tshark is not installed, a note reads "tshark is not installed; only summary counts are available" and only the summary counts appear. If there is no analyzable traffic, the tab says so.

### Packets

The Packets tab is a Wireshark-style list: No., Time, Source, Destination, Protocol, Len, and Info. Type a display filter (for example `http`, `dns`, or `ip.addr==10.0.0.1`) and click Apply; Clear resets it. The list is capped (a "showing first N" / "N+" marker appears when it is truncated). Click any packet to load its full decoded protocol tree under "Frame N".

### Download pcap

"Download pcap" (on the viewer header and as Download in the sessions table) saves the file to open in Wireshark.

## Tips

- Pair a capture with a pack deploy: start the capture, deploy a member that browses and replays a login, and the credentials appear in the Analysis tab within seconds.
- Use the HTTP or DNS preset to keep a capture small and focused for one lesson.
- Pick the Wireless layer when the goal is 802.11 frames or WPA handshakes rather than IP traffic.
