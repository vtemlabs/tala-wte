# Pixie-Dust hostapd (embedded)

Tala WTE ships a second, deliberately weakened `hostapd` so a WPS lab network can
be made vulnerable to the **Pixie-Dust** attack on demand. It is stock
**hostapd 2.11** with two changes:

1. **WPS enabled** (`CONFIG_WPS=y`) so the AP answers an external registrar.
2. **The patch in `pixie-dust.patch`** zeros the WPS enrollee/registrar secret
   nonces (E-S1/E-S2) instead of drawing them from the RNG.

With predictable nonces, `pixiewps` recovers the WPS PIN offline from a single
M1-M3 exchange. Stock hostapd uses a strong RNG for those nonces and cannot be
Pixie'd, so a normal WPS network here resists Pixie Dust and is only attackable
with the slower online PIN brute force (reaver/bully) - which is how a modern,
patched router behaves.

## How it is used

The built binaries live in `internal/sim/pixie/hostapd-amd64` and
`internal/sim/pixie/hostapd-arm64` and are embedded into the Tala WTE binary via
`go:embed`. When a WPS network has **Pixie-Dust Downgrade** enabled, the sim
extracts the architecture-matching binary to a temp path and runs it instead of
system hostapd. Every other network, and every WPS network without the
downgrade, uses system hostapd unchanged.

## Rebuilding

Run `build.sh` once on each architecture (it writes to `internal/sim/pixie/`):

```sh
# on an x86_64 host -> internal/sim/pixie/hostapd-amd64
# on an aarch64 host -> internal/sim/pixie/hostapd-arm64
sudo apt-get install -y build-essential pkg-config libnl-3-dev libnl-genl-3-dev libssl-dev wget patch
./build/hostapd/build.sh
```

The binaries are dynamically linked against `libnl-3` and `libssl`, which Tala
WTE already installs as hostapd dependencies, so they run on any box where Tala
WTE is installed.
