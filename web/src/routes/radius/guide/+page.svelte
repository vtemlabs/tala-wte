<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, paid training, paid CTF,
  or any for-profit use requires a license from VTEM Labs. See the LICENSE file.
-->
<script lang="ts">
  import GuidePage from '$lib/components/GuidePage.svelte';

  const DOC = `## Overview

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

Together these four pieces (a network on hostapd, this RADIUS server, the LDAP directory, and the certificates) form a complete, realistic enterprise Wi-Fi authentication stack you can attack, defend, and demonstrate.`;
</script>

<GuidePage
  title="RADIUS Guide"
  subtitle="The 802.1X authentication server for WPA-Enterprise networks"
  backHref="/radius"
  backLabel="Back to RADIUS"
  crumb="RADIUS"
  doc={DOC}
/>
