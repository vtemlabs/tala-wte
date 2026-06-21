The Tala WTE HTTP API is the same one the web console uses; everything you can do in the browser, you can drive with a request. There is no separate or undocumented surface. It is built for a local, isolated training lab and is not meant to be exposed to the internet.

The API has three layers:

- The PocketBase **Collections API** under `/api/collections/...` for stored data (networks, portals, captures, and so on). These are locked to superusers only.
- The Tala WTE **app API** under `/api/wte/...` for actions (start a network, run a capture, provision LDAP, drive a pack member, update the box). Most of these require a superuser; a handful accept a pack leader's agent key, and a few are public by design.
- The first-boot **setup** endpoints, which exist only until the admin account is created.

For what the routes do in context, see [[Networks]], [[Captive-Portals]], [[Packet-Captures]], [[RADIUS-802.1X]], [[Client-Mode]], [[The-Pack]], and [[Settings]]. To get a box to the point where the API is reachable, see [[Installation]] and [[System-Requirements]].

## Base URL and TLS

The API is served over HTTPS on port 8443:

```
https://<host>:8443
```

The certificate is self-signed (a built-in CA issues it on first boot), so clients must skip CA verification or trust the box's CA. Port 80 exists only to redirect to 8443. PocketBase itself binds to `127.0.0.1:8090` behind the TLS proxy and is not meant to be reached directly.

This is a lab tool. Do not put it on the public internet.

## Authentication model

There are three ways a request is authorized, and each endpoint uses exactly one.

**Superuser token (most endpoints).** Authenticate against the PocketBase superusers collection and send the returned token as the `Authorization` header.

```
POST /api/collections/_superusers/auth-with-password
Content-Type: application/json

{ "identity": "admin@example.com", "password": "..." }
```

The response carries a `token` field. Send it on later requests:

```
Authorization: <token>
```

Endpoints marked **superuser** below reject anything that is not an authenticated superuser with HTTP 403 (`{"error":"superuser authentication required"}`). Note that being any authenticated record is not enough; the wrapper checks for superuser specifically.

**Agent key (pack control endpoints).** A pack member exposes a long random agent key. A pack leader presents it in the `X-Agent-Key` header to drive that member remotely over its self-signed HTTPS. Endpoints marked **agent-key** accept either a local superuser (the member's own console) or a matching agent key; anything else gets HTTP 403 (`{"error":"agent key or superuser auth required"}`). The member's own console retrieves and rotates this key through the client endpoints below.

**Public (no auth).** A few endpoints serve content that must be reachable before an admin exists or that an iframe cannot send headers for. They are marked **public** below and serve only non-sensitive content.

The in-browser terminal is a special case: WebSockets cannot send the `Authorization` header, so it takes the superuser token as the `?token=` query parameter instead.

## Responses and errors

App endpoints (`/api/wte/...`) return indented JSON. Errors use the shape `{"error":"message"}` with an appropriate HTTP status. Successful action endpoints typically return a small status object such as `{"status":"started","id":"..."}`. The Collections API uses PocketBase's own response and error formats.

## Authentication & Setup

First-boot account setup runs entirely in the browser wizard. No superuser is auto-created and no credentials are printed; instead a one-time setup token is logged to the server journal at first boot (look for the `SETUP TOKEN:` line). The wizard endpoints are unauthenticated because no admin exists yet, and `/complete` hard-rejects once a real admin is present.

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/api/wte/setup/status` | public | Report whether first-boot setup is still needed. |
| POST | `/api/wte/setup/complete` | public | Create the first superuser from the wizard. |

`GET /api/wte/setup/status` returns `{"needs_setup": <bool>}`. While setup is still needed it (re)generates and logs the setup token.

`POST /api/wte/setup/complete` body:

```json
{
  "email": "admin@example.com",
  "password": "at-least-10-chars",
  "setup_token": "<token from the server log>",
  "license_ack": true
}
```

The password must be at least 10 characters, the email must contain `@`, the setup token must match the logged one, and `license_ack` must be `true` (the license gate is enforced server-side). On success it returns an auth token shaped like a normal PocketBase login so the browser can persist it:

```json
{
  "token": "<auth token>",
  "record": { "id": "...", "email": "...", "collectionName": "_superusers" }
}
```

Once a real admin exists this endpoint returns HTTP 409 and you log in through the normal `auth-with-password` flow instead.

## Networks

Networks are AP definitions. The records live in the `networks` collection (see the Collections API); these endpoints act on a saved network by its record id. See [[Networks]].

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/api/wte/networks/{id}/start` | superuser | Bring a network online (hostapd + DHCP + optional portal). |
| POST | `/api/wte/networks/{id}/stop` | superuser | Stop a running network and tear down its resources. |
| GET | `/api/wte/networks/{id}/status` | superuser | Report whether the network is running, stopped, or errored. |
| GET | `/api/wte/networks/{id}/clients` | superuser | List clients currently associated to the network. |
| GET | `/api/wte/networks/{id}/logs` | superuser | Return the per-network activity log lines. |
| GET | `/api/wte/networks/{id}/client-config` | superuser | Download a client connection profile (`.json`) for this network. |

`start` accepts an optional JSON body; all fields are optional:

```json
{ "auto_provision": false, "interface": "wlan1", "band": "5" }
```

`interface` and `band` override the saved adapter and band (used when the radio-management prompt confirms a substitute adapter). `auto_provision` applies only to enterprise SSIDs: if the enterprise preflight fails and this is `true`, the box runs the enterprise auto-provision before starting. Responses include `{"status":"started","id":...}` on success, `{"status":"already_running"}` if it was already up, HTTP 412 with a `preflight` report when an enterprise SSID is not ready and `auto_provision` was not set, and HTTP 500 with `provision` and `preflight` reports if auto-provision fails.

`stop` and `status` return `{"status":...,"id":...}`. `clients` returns `{"clients":[...]}`. `logs` returns `{"lines":[...]}`. `client-config` streams a downloadable JSON profile (the same shape a Tala WTE client imports) as a file attachment.

## Captive Portals & Credentials

Portal templates and generated credential sets live in the `portals` and `portal_credentials` collections; these endpoints manage the catalog and the credential generator. See [[Captive-Portals]].

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/api/wte/portals/auth-types` | superuser | List captive-portal auth types and the fields each collects. |
| POST | `/api/wte/portals/credentials/generate` | superuser | Generate a validatable credential set for an auth type. |
| GET | `/api/wte/portals/templates` | superuser | List the built-in template gallery. |
| POST | `/api/wte/portals/restore` | superuser | Re-seed built-in templates (recreate deleted, reset changed). |
| POST | `/api/wte/portals/upload` | superuser | Upload a portal as a single `.html` file or a `.zip` bundle. |
| POST | `/api/wte/portals/scrape` | superuser | Clone a live page by URL into a custom portal (SSRF-guarded). |
| GET | `/api/wte/portals/{id}/preview` | public | Render a portal for in-UI iframe preview (same-origin framed). |
| GET | `/api/wte/portals/{id}/preview/{path...}` | public | Serve an asset inside a multi-file portal bundle for preview. |

`auth-types` returns `{"auth_types":[...]}`. `templates` returns `{"templates":[...]}`.

`credentials/generate` body:

```json
{ "name": "Hotel set", "auth_type": "hotel", "count": 25 }
```

The auth type must be one that uses credentials. `count` defaults to 25 and is capped at 1000. It saves a `portal_credentials` record and returns `{"id":...,"name":...,"count":...}`.

`upload` is a multipart form with a `file` field (a `.html`/`.htm` document, or a `.zip` bundle that contains an `index.html` at its root) and an optional `name` field; max 25 MB. It returns `{"id":...,"name":...}`.

`scrape` body:

```json
{ "url": "https://example.com/login", "name": "Example portal" }
```

`url` is required; `name` defaults to the page host. It returns `{"id":...,"name":...}`.

`restore` returns `{"restored":<n>,"reset":<n>}`. The preview endpoints are public because an iframe cannot attach the auth header; they serve only same-origin-framed, non-sensitive portal content.

## Captures

Packet captures run against a network and are recorded in the `captures` collection; the record id doubles as the capture session id. See [[Packet-Captures]].

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/api/wte/captures/start` | superuser | Start a packet capture on an interface for a network. |
| POST | `/api/wte/captures/{id}/stop` | superuser | Stop a capture and finalize the pcap and packet count. |
| GET | `/api/wte/captures/{id}/download` | superuser | Download the finished pcapng file. |
| GET | `/api/wte/captures/{id}/analyze` | superuser | Return a structured analysis of the capture. |
| GET | `/api/wte/captures/{id}/packets` | superuser | Return the packet list, optionally filtered. |
| GET | `/api/wte/captures/{id}/packet/{n}` | superuser | Return the full dissection of one frame. |

`start` body:

```json
{ "network_id": "...", "interface": "wlan1", "layer": "network", "filter": "" }
```

`network_id` and `interface` are required. `layer` is `network` (default) or `wireless`. `filter` is an optional BPF capture filter (validated). It returns `{"status":"started","id":...,"file":...}`.

`stop` returns `{"status":"stopped","id":...,"packet_count":<n>}`. `download` streams the `.pcapng` (content type `application/vnd.tcpdump.pcap`) as an attachment. `analyze` returns the analysis object (protocol mix, top talkers, DNS, HTTP, cleartext credentials). `packets` accepts `?filter=<wireshark display filter>` and `?limit=<n>` and returns `{"packets":[...],"truncated":<bool>,"count":<n>}`. `packet/{n}` returns `{"detail":"..."}` for frame number `n`.

## LDAP

The embedded OpenLDAP directory backs WPA-Enterprise auth. These endpoints manage users, groups, and the directory as a whole. See [[RADIUS-802.1X]].

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/api/wte/ldap/users` | superuser | List directory users. |
| POST | `/api/wte/ldap/users` | superuser | Create a user. |
| PUT | `/api/wte/ldap/users/{uid}` | superuser | Update a user's attributes. |
| DELETE | `/api/wte/ldap/users/{uid}` | superuser | Delete a user. |
| POST | `/api/wte/ldap/users/{uid}/password` | superuser | Set a user's password. |
| GET | `/api/wte/ldap/groups` | superuser | List groups. |
| POST | `/api/wte/ldap/groups` | superuser | Create a group. |
| DELETE | `/api/wte/ldap/groups/{cn}` | superuser | Delete a group. |
| POST | `/api/wte/ldap/groups/{cn}/members` | superuser | Add a user to a group. |
| DELETE | `/api/wte/ldap/groups/{cn}/members/{uid}` | superuser | Remove a user from a group. |
| POST | `/api/wte/ldap/test-auth` | superuser | Test an LDAP bind for a user. |
| GET | `/api/wte/ldap/status` | superuser | Report slapd status and base DN. |
| POST | `/api/wte/ldap/provision` | superuser | Wipe and reprovision the directory from a company template. |
| POST | `/api/wte/ldap/provision/random` | superuser | Wipe and reprovision with a random company and user count. |

`GET /users` returns `{"users":[...]}`, where each user carries `uid`, `cn`, `sn`, `given_name`, `mail`, optional `title` and `department`, `groups`, and `dn`. `POST /users` body: `uid`, `cn`, `sn`, `given_name`, `mail`, `password`; returns `{"status":"created","dn":...}`. `PUT /users/{uid}` body is a map of attribute updates (limited to `cn`, `sn`, `givenName`, `mail`); returns `{"status":"updated","uid":...}`. `DELETE /users/{uid}` returns `{"status":"deleted","uid":...}`. `POST /users/{uid}/password` body `{"password":"..."}`; returns `{"status":"password_set","uid":...}`.

`GET /groups` returns `{"groups":[...]}` with `cn`, `members`, `dn` per group. `POST /groups` body `{"cn":"...","members":[...]}` (members optional); returns `{"status":"created","cn":...}`. `DELETE /groups/{cn}` returns `{"status":"deleted","cn":...}`. `POST /groups/{cn}/members` body `{"uid":"..."}`; returns `{"status":"member_added","cn":...,"uid":...}`. `DELETE /groups/{cn}/members/{uid}` returns `{"status":"member_removed","cn":...,"uid":...}`.

`test-auth` body `{"uid":"...","password":"..."}`; returns `{"success":true,"dn":...}` on a successful bind or `{"success":false,"message":...}` otherwise. `status` returns `{"running":<bool>,"base_dn":"..."}`.

`provision` body:

```json
{ "company_name": "ACME Corp", "domain": "acme.example", "user_count": 25, "random_passwords": true }
```

Both provision endpoints wipe the directory and reprovision it, returning `{"status":"provisioned","company_name":...,"domain":...,"users":[...]}` where each user carries `uid`, `cn`, `mail`, `password`, `department`, and `title`. `provision/random` takes no body and picks a random company and user count.

## RADIUS

FreeRADIUS configuration for enterprise SSIDs. The per-network RADIUS settings live in the `radius_config` collection; this endpoint applies the global server settings. See [[RADIUS-802.1X]].

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/api/wte/radius/config` | superuser | Save and apply FreeRADIUS settings, then restart the service. |

Body:

```json
{ "eap_type": "peap", "inner_auth": "mschapv2", "shared_secret": "..." }
```

All fields are optional (only the supplied ones are applied). The shared secret is character-validated, written to `clients.conf`, and the EAP method is applied to the running FreeRADIUS before it is restarted. Returns `{"status":"saved","eap_type":...,"inner_auth":...}`.

## Certificates

The internal PKI issues the CA and the certificates enterprise networks use. The on-disk PKI is mirrored into the `certificates` collection for the UI. See [[RADIUS-802.1X]].

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/api/wte/certs/ca` | superuser | Initialize a new internal CA. |
| POST | `/api/wte/certs/server` | superuser | Issue a CA-signed server certificate. |
| POST | `/api/wte/certs/client` | superuser | Issue a CA-signed client certificate (EAP-TLS). |

These read parameters from the query string, not a body. `ca` returns `{"status":"ca_created","pki_dir":...}`. `server` accepts `?name=` (defaults to `radius-server`) and returns `{"status":"server_cert_created","name":...}`. `client` requires `?uid=` and returns `{"status":"client_cert_created","uid":...}`. Names and uids must be alphanumeric plus dash and underscore. After any of these, the certificates collection is reconciled so the new material shows up immediately.

## Enterprise

Readiness checks and one-shot provisioning that bring every WPA-Enterprise dependency (LDAP, CA, server cert, FreeRADIUS modules and config) to a known-good state. See [[RADIUS-802.1X]].

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/api/wte/enterprise/preflight` | superuser | Return the enterprise readiness checklist (no mutations). |
| POST | `/api/wte/enterprise/provision` | superuser | Run the auto-provision and return the per-step report. |

`preflight` returns `{"ok":<bool>,"checks":[{"id","label","ok","detail","auto_fixable"}, ...]}`. `provision` takes no body and returns `{"ok":<bool>,"steps":[{"id","label","status","detail"}, ...]}` and, when LDAP was provisioned, a `users` array.

## Client mode

When a box is switched to client mode it joins a target AP and generates traffic. These endpoints connect, drive traffic, and report status. They are **agent-key** endpoints: the member's own superuser console or a pack leader's agent key may call them, so a leader can drive a member remotely. The agent-key retrieval and rotation endpoints are superuser-only (the member sets up its own key). See [[Client-Mode]].

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/api/wte/client/connect` | agent-key | Connect to an imported network profile. |
| POST | `/api/wte/client/start` | agent-key | Start the selected traffic generators. |
| POST | `/api/wte/client/stop` | agent-key | Stop traffic but keep the connection up. |
| POST | `/api/wte/client/disconnect` | agent-key | Stop traffic and tear down the Wi-Fi connection. |
| GET | `/api/wte/client/status` | agent-key | Return live client status. |
| POST | `/api/wte/client/reconnect` | agent-key | Toggle reconnect cycling (handshake capture). |
| GET | `/api/wte/client/logs` | agent-key | Return the client's activity log. |
| GET | `/api/wte/client/agent-key` | superuser | Return this member's agent key (creating one if needed). |
| POST | `/api/wte/client/agent-key/regenerate` | superuser | Rotate the agent key. |

`connect` takes a client config profile (the JSON exported from a network's `client-config` endpoint, with at least an `ssid`); the join runs in the background and the response is `{"status":"connecting","ssid":...}`. `start` takes traffic options (the generator toggles) and returns the live status; `stop`, `disconnect`, and `status` return the live status object. `reconnect` body `{"enabled":<bool>,"frequency_seconds":<n>,"jitter_seconds":<n>}` controls periodic deauth/reassociate for WPA handshake capture. `logs` returns `{"lines":[...]}`. `agent-key` and `agent-key/regenerate` return `{"key":"..."}`.

## Pack

A pack leader (an AP box) drives registered client members. Members are stored in the `pack_members` collection; the leader reaches each over its pinned, self-signed channel using the member's agent key. See [[The-Pack]].

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/api/wte/pack/{id}/deploy` | superuser | Assign a member to a network and bring it online. |
| POST | `/api/wte/pack/{id}/stop` | superuser | Stop traffic, disconnect the member, clear its assignment. |
| GET | `/api/wte/pack/{id}/status` | superuser | Proxy the member's live status through the leader. |
| POST | `/api/wte/pack/update` | superuser | Update every pack member from the leader. |
| GET | `/api/wte/pack/discovered` | superuser | Browse the LAN over mDNS for other Tala WTE instances. |

`{id}` is the `pack_members` record id. `deploy` body:

```json
{ "network_id": "...", "traffic": { ... }, "reconnect": { "enabled": true, "frequency_seconds": 60, "jitter_seconds": 10 } }
```

`network_id` is required; `traffic` is the generator mix (defaults to a standard web/DNS/ping/local/internet mix) and `reconnect` is the optional handshake-cycling schedule. The leader pushes the network's profile to the member, then starts traffic in the background once it associates; the response is `{"status":"deploying"}`. `stop` returns `{"status":"stopped"}`. `status` returns `{"reachable":<bool>,"status":{...}}` (or `{"reachable":false,"error":...}` if the member cannot be reached). `update` downloads each needed CPU architecture once and pushes the matching, checksum-verified binary to each member (falling back to a member pulling from GitHub), returning `{"results":[{"name","ok","detail"}, ...]}`. `discovered` returns `{"peers":[...]}`, filtering out this instance.

## System & Settings

Host status, interfaces, role swap, settings, the in-browser terminal, and the self-update. See [[Settings]].

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/api/wte/system/status` | public | Aggregate system status (used by the login/setup screen). |
| GET | `/api/wte/system/interfaces` | superuser | List wireless adapters (free, in-use, unsupported). |
| POST | `/api/wte/system/interfaces/heal` | superuser | Run a USB-reset recovery on a wedged adapter. |
| POST | `/api/wte/system/mode` | superuser | Swap the instance between AP and client roles. |
| GET | `/api/wte/system/version` | superuser | Report running version and whether an update exists. |
| POST | `/api/wte/system/update` | agent-key | Pull and apply the latest release, then restart. |
| POST | `/api/wte/system/apply` | agent-key | Apply a binary pushed by a pack leader, then restart. |
| GET | `/api/wte/system/settings` | superuser | Read uplink interface, regulatory domain, AP subnet. |
| POST | `/api/wte/system/settings` | superuser | Save those settings. |
| GET | `/api/wte/terminal/ws` | superuser | PTY shell over a WebSocket (token via `?token=`). |

`system/status` is public so the login and setup screens can render before an admin exists; it returns `{"status":"ok","mode":...,"needs_setup":<bool>,"radius_running":<bool>,"ldap_running":<bool>,"interface_count":<n>,"real_adapter_count":<n>}`.

`system/interfaces` returns `{"interfaces":[...],"in_use":{iface:ssid},"in_use_adapters":[...],"unsupported":[...]}`. `interfaces/heal` body `{"interface":"wlan1"}` or `{"usb_path":"..."}` (one is required); returns `{"ok":true,"message":...}`.

`system/mode` body `{"mode":"ap"}` or `{"mode":"client"}`; it persists the role and restarts the service in the background, returning `{"status":"switching","mode":...}` first. `system/version` returns the version status object: `current`, `latest`, `update_available`, `notes`, `release_url`, `is_dev`, and an optional `error`.

`system/update` and `system/apply` are agent-key so a pack leader can update a member with its agent key. `update` pulls the latest release from GitHub, verifies the checksum, swaps the binary, and schedules a restart, returning `{"status":"updating","version":...,"restarting":true,"message":...}`. `apply` receives a binary streamed by a leader (with `X-Update-SHA256`, `X-Update-Version`, and `X-Update-Arch` headers; the body limit on this route is raised to 256 MB), verifies it, and restarts.

`GET /system/settings` returns `{"uplink_iface":...,"country_code":...,"ap_subnet":...}`. `POST /system/settings` body:

```json
{ "uplink_iface": "enp0s5", "country_code": "US", "ap_subnet": "10.0.0.0/24" }
```

All fields are optional. The country code must be two letters and is applied to the live radio's regulatory domain; the subnet must be CIDR. Returns `{"status":"saved", ...}` echoing the saved values.

`terminal/ws` upgrades to a WebSocket carrying a full PTY login shell. Because WebSockets cannot send the `Authorization` header, the superuser token is passed as `?token=`; origin is restricted to same-origin. Binary frames are shell output; a JSON text frame `{"type":"resize","cols":...,"rows":...}` resizes the PTY.

## License

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET | `/api/wte/license` | public | Serve the Tala WTE license as plain text. |

This is public because the license must be readable before an admin account is created (the setup wizard shows it). The related legal pages `/legal/terms`, `/legal/aup`, and `/legal/privacy` are also served publicly so portal-preview links resolve.

## Collections API

Stored data is reached through PocketBase's standard records API. Each managed collection supports the usual operations:

```
GET    /api/collections/<name>/records          list + filter + sort + paginate
GET    /api/collections/<name>/records/<id>     view one
POST   /api/collections/<name>/records          create
PATCH  /api/collections/<name>/records/<id>     update
DELETE /api/collections/<name>/records/<id>     delete
```

Every managed collection has its API rules locked, so **only an authenticated superuser** can read or write them; send the superuser token in the `Authorization` header. The managed collections are:

| Collection | Holds |
|---|---|
| `networks` | AP/SSID definitions (protocol, band, channel, portal binding, status). |
| `portals` | Captive-portal templates (built-in and custom). Built-ins are read-only; an edit request is rejected, so clone one to customize it. |
| `portal_credentials` | Generated credential sets a portal validates submissions against. |
| `portal_submissions` | Credentials/data captured from clients at a portal. |
| `captures` | Packet-capture sessions (interface, filter, status, packet count). |
| `certificates` | Mirror of the on-disk PKI (CA, server, client) for the UI. |
| `clients` | Clients seen associated to networks. |
| `radius_config` | Per-network RADIUS settings (EAP type, inner auth, cert relations). |
| `traffic_datasets` | Named URL/domain/IP target sets for traffic generation. |
| `pack_members` | Registered client members a leader drives (address, agent key, pinned fingerprint). |
| `client_configs` | Saved client connection profiles (used in client mode). |
| `settings` | Internal key/value store (agent key, persisted settings, setup token). |

For login itself, the superusers collection uses the auth endpoint shown earlier: `POST /api/collections/_superusers/auth-with-password`. The default public `users` collection that PocketBase ships with is removed (or locked) at boot, so there is no public self-registration.

## Related pages

- [[Installation]] - get the box up so the API is reachable
- [[System-Requirements]] - supported hardware and OS
- [[Networks]] - what the network endpoints drive
- [[Captive-Portals]] - portals and credential sets
- [[Packet-Captures]] - packet capture and analysis
- [[RADIUS-802.1X]] - LDAP, RADIUS, and certificates together
- [[Client-Mode]] - the client endpoints in context
- [[The-Pack]] - driving members from a leader
- [[Settings]] - system settings, role swap, and the agent key
