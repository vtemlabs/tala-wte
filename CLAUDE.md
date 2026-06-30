# Tala WTE

Wireless Training Environment (WTE) by VTEM Labs. Single Go binary that turns one Linux host with a Wi-Fi adapter into a vulnerable-by-design wireless lab: real hostapd access points across every 802.11 security protocol, captive portals, a full WPA-Enterprise stack, traffic-generating clients, and in-app capture/analysis. Authorized offensive-security training/dev tooling; lab-only, not hardened.

Relationship to `tala`: WTE is the open, safe training/development counterpart to TALA (the VTEM Labs professional wireless pentest platform). WTE covers only 802.11, a subset of TALA's multi-modal RF scope. Sibling repos, not shared code; module is `github.com/vtemlabs/tala-wte`.

## Stack
Go 1.26 backend on PocketBase 0.37 (embedded SQLite via pure-Go `modernc.org/sqlite`, realtime, superuser auth). Frontend is SvelteKit 2 / Svelte 5 (runes) / TS 5 / Vite 6 with `adapter-static`, built to `web/build` and embedded. Linux only (Debian/Ubuntu/Kali), root required. System services driven as deps: hostapd, dnsmasq, iptables/iproute2, FreeRADIUS, OpenLDAP, tshark/tcpdump.

## Runtime topology
`main()` (`cmd/server/main.go`): forces pb_data to `/var/lib/tala-wte` (injects `--dir=`, avoids Parallels shared folders), verifies/installs apt deps every boot, then PocketBase serves on loopback `127.0.0.1:8090`. Public front door is a self-signed TLS reverse proxy on `:8443` plus `:80` -> HTTPS redirect (`spawnSecureProxies`). Two roles in one binary: AP/server (default, blue UI) and client (orange UI), selected by `/var/lib/tala-wte/mode` file or `TALA_MODE` env. A server can be a pack leader driving client members.

## Layout
- `ui.go` - top-level `package tala`; `go:embed all:web/build` (FrontendFS) + `go:embed LICENSE`.
- `cmd/server/` - the only `package main`, one binary, split by concern: `main.go` (proxy + PocketBase wiring + route registration), `handlers.go`, `bootstrap.go` (collections), `setup.go` (first-boot admin), `terminal.go` (PTY-over-WS), `install.go`, `pack.go`, `client.go`, `discovery.go`, `update.go`, `certs_reconcile.go`, `eterminal.go`, `autostart.go`, `enterprise_startup.go`, `portal_credentials.go`.
- `internal/sim/` - network lifecycle manager (hostapd AP start/stop/status/clients/logs), preflight, enterprise auto-provision, netlog, teardown.
- `internal/sim/pixie/` - `go:embed` of TRACKED prebuilt `hostapd-amd64`/`hostapd-arm64` (patched hostapd 2.11: zeroed WPS E-S1/E-S2 for pixiewps, RSN PMKID in EAPOL 1/4, CONFIG_WEP). Used only by networks opting into `wps_pixie`/`pmkid_exposed`/`wep_real`; all others use system hostapd.
- `internal/portal/` - captive-portal engine, `authtype.go` catalog, `catalog.go` (`go:embed templates/*.html`, 36 portals), scrape, normalize, legal pages.
- `internal/ldap/` - embedded OpenLDAP (slapd): users, groups, auth, provision.
- `internal/certs/` - CA + server/client cert issuance (EAP-TLS PKI under `CADir()`).
- `internal/client/` - client-mode agent: join AP, six traffic generators, handshake-capture reconnect cycling.
- `internal/iface/` - adapter discovery, heal wedged USB radios, unsupported-adapter detection.
- `internal/routing/` - dnsmasq (DHCP + captive DNS) and NAT/iptables.
- `internal/netns/` - Linux network namespaces for per-AP isolation.
- `internal/capture/` - pcap session start/stop + in-place analyzer/viewer.
- `internal/deps/` - apt verify/install, OS detect, USB Wi-Fi recovery.
- `internal/discovery/` - mDNS advertise/browse on `_tala-wte._tcp` for pack discovery.
- `internal/updater/` - GitHub release check + self-update binary swap.
- `internal/eterminal/` - `go:embed all:assets` of vendored e-terminal (gitignored, pulled fresh by `make eterminal`; `HasInstaller()` gates it).
- `internal/version/`, `internal/api/` (JSON/error helpers).
- `pkg/hostapd/` - hostapd config generation + process supervision (the one exported pkg).
- `web/` - SvelteKit app; routes: networks, portals, ldap, radius, captures, certificates, client, pack, settings, login.
- `build/hostapd/` - `build.sh` + `pixie-dust.patch` to rebuild the embedded patched hostapd.

## Public surface
CLI: `install`, `install-client`, `uninstall [--purge]` are intercepted by `maybeRunSubcommand` and exit before PocketBase parses args; any other verb (e.g. `serve --http`) falls through to the PocketBase cobra root. install copies the binary to `/var/lib/tala-wte`, writes/starts `tala-wte.service`, preserves the DB across reinstalls (also the upgrade path), never creates an account.

HTTP (PocketBase router, prefix `/api/wte/`): `GET /{path...}` serves the embedded SvelteKit build; `/legal/{terms,aup,privacy}`. Groups: `setup/*` + `license` (unauth, first-boot only); `terminal/ws` (WS, superuser via `?token=`); `networks/{id}/*`; `client/*` + `pack/*`; `enterprise/{preflight,provision}`; `portals/*` (`{id}/preview` unauth for iframes); `captures/*`; `certs/{ca,server,client}`; `ldap/*`; `system/*`; `radius/config`. Auth wrappers: `wrapAuth` = superuser only; `wrapAgent` = local superuser OR a pack-leader agent key; intentionally unauth: setup, license, `system/status`, portal preview. `system/apply` raises the body limit to 256 MB for pushed release binaries.

## Build (Makefile)
- `make build` = `build-web` (pnpm build to `web/build`) + `build-go` (native, CGO default).
- `make linux` / `linux-amd64` / `linux-arm64` - cross-builds, `CGO_ENABLED=0` pure-Go, `-trimpath -ldflags "-s -w"`, version from git tag; depend on `build-web` + `eterminal`.
- `make eterminal` - clones latest e-terminal into `internal/eterminal/assets/e-terminal` (trimmed to Linux), keeps existing copy on failure.
- `release` = web + both Linux binaries. `run` = `sudo dist/tala-wte serve --http=0.0.0.0:8090`. `lint`/`fmt` cover Go + web; `lint-go` stubs `web/build/index.html` so the go:embed package loads.
- Version: `internal/version.Version` stamped via ldflags from nearest git tag; `"dev"` locally suppresses update checks.

## Gotchas
- Public listener is `:8443` (TLS proxy), NOT `8090` (loopback only).
- Pixie hostapd binaries are committed; rebuild from `build/hostapd/`. e-terminal vendor dir and `kernel/` (private driver-fix bundle) are gitignored and absent from clean checkouts.
- Built-in portal templates are read-only at the API (`type=builtin` edits rejected; UI clones to edit).
- Boot auto-restores prior running state (`bootAutoStart`): AP restarts running networks, client reconnects last network; do not reset network statuses on shutdown.
- Vulnerable-by-design: deliberately insecure networks and weak credentials. Run only on an isolated lab uplink, never internet-facing.
- README stale: it says Go 1.25 / PocketBase 0.36; actual is Go 1.26 / PocketBase 0.37 (go.mod).
