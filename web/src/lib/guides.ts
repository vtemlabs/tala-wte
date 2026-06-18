// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government use
// require a license from VTEM Labs. See the LICENSE file.

// Shared in-app guide content. Each guide can be shown full-page (the /*/guide
// routes) or in the floating, adjustable Guide window so it can be referenced
// while adjusting settings and running tasks.

export interface Guide {
  title: string;
  subtitle?: string;
  doc: string;
}

export const GUIDES: Record<string, Guide> = {
  client: {
    title: 'Client Mode',
    subtitle: 'Run this box as a traffic-generating client',
    doc: `## Overview

In **client mode** a Tala WTE box joins another Tala WTE access point as a station and behaves like a real device on the network: it pulls a DHCP lease, gets past a captive portal, and generates realistic traffic. Use it to populate a training network with believable activity, to feed packet captures, and to give trainees credentials and handshakes to capture.

## Dashboard

The client **Dashboard** is the landing page. It shows:

- **Connection** - online/offline, the joined SSID, and the leased IP.
- **Connection panel** - SSID, interface, IP, gateway, and captive-portal state.
- **Wireless Interfaces** - the adapter(s) this client can use to associate.
- **Traffic Generation** - whether traffic is running, with request / received / error counters.

## Switching roles

A box can be either an access point (server) or a client. Flip it any time from **Settings -> Instance Role** with one button - it installs the other role's dependencies and restarts into the new mode. See the Settings guide.

## Joining the den

A client can be driven remotely by a den leader. Copy its **Den Agent Key** from Settings and register it on the leader's **Den** page; the leader can then push a network config and start traffic for you. See the Den guide.`
  },

  traffic: {
    title: 'Traffic Console',
    subtitle: 'Join a network and generate realistic traffic',
    doc: `## Saved networks

Upload a **client config** (exported from any access point's network detail page) by dropping it on the upload zone or clicking to browse. Each upload is **saved**, so you can keep a library of networks and jump between them at any time. Click **Connect** on a saved network to join it; the client associates, pulls DHCP, and auto-bypasses a captive portal if the config has one. **Disconnect** drops the link.

## Traffic generation

Pick which generators run, then **Start traffic**:

- **Web browsing** - HTTP/HTTPS GETs.
- **DNS lookups** - background name resolution.
- **Ping / local LAN** - ICMP and intra-LAN chatter.
- **Downloads / bandwidth** - periodic larger transfers.
- **Credential logins** - replays the logins you list below in cleartext (HTTP Basic + form POST) so trainees can capture them.
- **Domain chatter (responder bait)** - LLMNR / NBT-NS / mDNS name lookups, the bait responder-style poisoning attacks feed on.

**Target scope** chooses local (gateway and LAN) and/or internet hosts.

## Targets and credentials

Provide your own **URLs**, **domains**, and **IPs** (one per line) so traffic hits hosts you control, and add **login credentials** the client replays. Credentials are sent in cleartext on purpose - capturing and decrypting them is the exercise.

## Handshake capture

**Reconnect cycling** periodically deauthenticates and reassociates so students can capture a fresh WPA handshake each cycle. Set a **frequency** and a random **jitter** (presets or custom, seconds to hours) and click **Start cycling**.

## Live Log

**Live Log** opens a draggable, resizable window with the full client activity log - the connection lifecycle, what traffic started, and every error - so you can watch what the client is doing while you work.`
  },

  den: {
    title: 'The Den',
    subtitle: 'Drive a pack of client members from one leader',
    doc: `## Overview

The **den** lets an access-point server (the **den leader**) drive a pack of client instances (**members**). The leader pushes a network config to each member, starts its traffic, and stops it - all without logging into the members.

## Agent keys

Each member exposes a **Den Agent Key** in its own **Settings -> Den Agent Key**. Copy that key. The leader uses it to authenticate to the member's control API, so no member login is needed. Regenerating a member's key revokes any leader still holding the old one.

## Registering a member

On the leader's **Den** page, **Add member** with:

- **Name** - any label.
- **Address** - the member's host or host:port (https and :8443 are assumed).
- **Agent key** - pasted from the member.

The member then shows as **Reachable** with live status.

## Deploying

Pick a **network** and a **profile**, then **Deploy**:

- **Standard traffic** - web, DNS, ping over local + internet.
- **Full traffic** - every generator, including credential logins and responder bait.
- **Handshake capture** - traffic plus reconnect cycling for handshake capture.

The leader pushes the network's config to the member, waits for it to associate, and starts the chosen traffic. **Stop** disconnects the member.

## Teardown propagation

When you stop or delete a network on the leader, every member assigned to it is automatically disconnected, so members never chase a network that no longer exists.`
  },

  settings: {
    title: 'Settings',
    subtitle: 'Instance role, radio, uplink, and updates',
    doc: `## Instance role

Switch this box between **Server (AP)** and **Client** with one button. Switching installs the other role's dependencies and restarts the service into the new mode; the console reconnects automatically. Server mode broadcasts networks; client mode joins one and generates traffic.

## Den Agent Key (client mode)

In client mode, the **Den Agent Key** is the control token a den leader uses to drive this client. Copy it into the leader's Den page, or regenerate it to revoke access.

## Radio & Network

- **Regulatory Domain** - the country hostapd advertises (applied with \`iw reg set\`). It decides which channels are legal and whether 5 GHz / 6 GHz AP mode is allowed; it must match where the box operates.
- **Uplink Interface** - the interface connected to the internet, used for NAT on networks that allow passthrough.
- **Default Network Subnet** - the LAN/CIDR handed to clients that join a network (gateway \`.1\`, DHCP \`.10\`-\`.250\`). Each network can override it when created.

## Software Updates

Shows the installed and latest released versions. On a released build, updating downloads the verified binary, replaces the running service, and restarts it. Development builds disable in-place updates.`
  }
};
