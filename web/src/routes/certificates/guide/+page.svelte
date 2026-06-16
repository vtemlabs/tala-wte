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

WPA2-Enterprise and WPA3-Enterprise do not use a single shared passphrase. Instead, every client authenticates individually over **EAP** (the Extensible Authentication Protocol), brokered by a RADIUS server. EAP is built on TLS, and TLS is built on **X.509 certificates**. That is why this page exists: it is the certificate authority that issues and signs the certificates the rest of the enterprise stack depends on.

Two things always need a certificate:

- The **RADIUS server** must present a **server certificate** during the EAP handshake so clients can confirm they are talking to the genuine authentication server and not an impostor.
- For **EAP-TLS**, each user additionally needs a **client certificate**, which the user presents to prove their identity instead of typing a password.

A **certificate authority (CA)** is the trust anchor that signs both. A certificate is only trusted because the CA vouches for it, so nothing else on this page can be issued until the CA exists.

![The Certificates page](/guide/certificates.png)

---

## Step 1: Initialize the Certificate Authority

Until a CA exists, the Certificate Authority panel shows a **Required** badge and a single **Initialize CA** button. Click it once.

Initializing the CA produces a self-signed **root CA**. This root is the top of the trust chain: it signs every server and client certificate you issue afterward, and its signature is what makes those certificates verifiable. The page reflects the new state right away:

- The Certificate Authority panel flips to **Active** and lists the CA's **Name**, a **Type** of \`Root CA\`, and its **Expires** date.
- The header badge at the top of the page changes from **No CA** to **CA Ready**.
- The new CA appears in the **Certificates** table below with a type of \`ca\`.

> You initialize the CA once. From that point on, the **New Certificate** panel is what you use to issue everything else. There is no separate button to issue a CA from the form; the CA is created only through the Initialize CA action.

---

## Step 2: Issue a server certificate for FreeRADIUS

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
`;
</script>

<GuidePage
  title="Certificates Guide"
  subtitle="The certificate authority behind EAP and WPA-Enterprise"
  backHref="/certificates"
  backLabel="Back to Certificates"
  crumb="Certificates"
  doc={DOC}
/>
