FreeRADIUS is the gatekeeper for WPA2-Enterprise and WPA3-Enterprise (802.1X) networks. When a client does 802.1X, the access point hands the login to RADIUS, and RADIUS checks the user against the embedded LDAP directory. The RADIUS page configures how that handshake works.

The page is reached from the sidebar (RADIUS). A status badge in the header reads "FreeRADIUS running", "stopped", or "unknown". A stat strip shows the fixed ports and the LDAP backend:

- RADIUS Port: `1812` (authentication)
- Accounting Port: `1813` (accounting)
- LDAP Backend: `127.0.0.1:3389` (the embedded OpenLDAP / slapd, see [[LDAP-Directory]])

![The RADIUS page](images/radius.png)

## EAP configuration

![EAP configuration](images/radius-eap.png)

The EAP Configuration panel has three controls.

### Default EAP Type

This is the outer EAP method FreeRADIUS offers. Selecting it and saving actually reconfigures the running server: Tala WTE rewrites `default_eap_type` in the FreeRADIUS `eap` module and restarts the service. The same value is applied automatically during Auto-provision & Start on an enterprise network.

- EAP-PEAP (Recommended): the username and password travel inside a TLS tunnel. This is the common enterprise setup and the right default for a "corporate Wi-Fi login" lesson. The client only needs a username and password from the directory; the server presents a certificate.
- EAP-TLS (Certificate Auth): certificate-based, no password. Each user authenticates with a client certificate you issue on the Certificates page. Pick this only when the lesson is specifically about client-certificate authentication, because it needs a client cert per user (see [[Certificates]]).
- EAP-TTLS: another tunneled method. Like PEAP it carries an inner authentication, and it is the method that also allows a PAP inner. Pick this when you specifically want to demonstrate TTLS or a PAP inner exchange.

### Inner Authentication

The inner method used inside a tunneled EAP type (PEAP or TTLS):

- MSCHAPv2: the default, and what PEAP uses. Use this for the standard enterprise lab.
- PAP: a plaintext-inside-the-tunnel inner, available for TTLS. Use it only when the lesson calls for it.

The server validates either inner method against the directory.

### Shared Secret

The secret shared between the access point and RADIUS. Leave it blank to use the generated secret (Tala WTE creates and persists a random 32-character secret on first use), which is what you want for the built-in, self-contained setup. Set a value only if you need a specific known secret.

Click Save Configuration to apply: it persists the settings, rewrites `default_eap_type` in the eap module, writes the shared secret into `clients.conf` if you set one, and restarts FreeRADIUS so the changes take effect.

## The authentication chain

![The authentication chain](images/radius-chain.png)

The page draws the full path a login travels, so students can see where their credential goes:

```
Wi-Fi client -> EAP (PEAP/MSCHAPv2, or TLS with a client cert) ->
hostapd (AP) -> RADIUS Access-Request -> FreeRADIUS :1812 ->
LDAP bind -> OpenLDAP :3389 -> Access-Accept + MSK ->
4-way handshake -> connected
```

In words: the Wi-Fi client speaks EAP to hostapd (the access point), hostapd forwards a RADIUS Access-Request to FreeRADIUS on port 1812, FreeRADIUS binds the user against the LDAP directory on port 3389, and on success returns Access-Accept (with the keying material). The 4-way handshake then completes and the client is connected. The diagram updates to reflect the EAP Type and Inner Authentication you have selected.

From the chain panel you can jump straight to Manage LDAP Users ([[LDAP-Directory]]) and Manage Certs ([[Certificates]]), the other two pieces of the enterprise stack.

## You rarely need to touch this

For a basic enterprise lab the defaults work: EAP-PEAP with an MSCHAPv2 inner. On an enterprise network's Start, the preflight gate offers Auto-provision & Start (see [[Networks]]), which sets RADIUS up for you end to end: it ensures the CA and server certificate, provisions the LDAP directory if empty, wires the eap and ldap modules, applies your saved EAP type, validates the config, and restarts FreeRADIUS. You only come to this page when you want to change the EAP method (for example to teach EAP-TLS) or set a specific shared secret.

## Tips

- Leave the defaults (PEAP/MSCHAPv2) and let Auto-provision & Start do the setup for a standard enterprise lab.
- Use EAP-TLS only when the lesson is about client certificates; issue a client cert per user on the [[Certificates]] page first.
- Confirm a directory credential works on the LDAP Test Auth tab before relying on it for an enterprise login.
