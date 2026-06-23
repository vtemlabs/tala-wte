#!/usr/bin/env bash
# Tala WTE - Wireless Training Environment
# Copyright (c) 2026 VTEM Labs. All rights reserved.
#
# Builds the Pixie-Dust-vulnerable hostapd that Tala WTE embeds for the optional
# WPS "downgrade". It is stock hostapd 2.11 with WPS enabled and the WPS enrollee
# secret nonces (E-S1/E-S2) forced to zero, so pixiewps can recover the WPS PIN
# offline in seconds. Stock hostapd uses a strong RNG for those nonces and is
# not Pixie-vulnerable; this build is used ONLY for WPS lab networks that opt into
# the downgrade. Every other network uses system hostapd unchanged.
#
# Run it once on each target architecture: an x86_64 box produces hostapd-amd64,
# an aarch64 box produces hostapd-arm64. Output lands in internal/sim/pixie/,
# which is embedded into the Tala WTE binary via go:embed.
#
# Build deps (Debian/Ubuntu):
#   apt-get install -y build-essential pkg-config libnl-3-dev libnl-genl-3-dev libssl-dev wget
set -euo pipefail

VER=2.11
ARCH=$(uname -m)
case "$ARCH" in
  x86_64) GOARCH=amd64 ;;
  aarch64 | arm64) GOARCH=arm64 ;;
  *) echo "unsupported arch: $ARCH (need x86_64 or aarch64)" >&2; exit 1 ;;
esac

SRC="/tmp/hostapd-$VER"
DEST_DIR="$(cd "$(dirname "$0")/../../internal/sim/pixie" && pwd)"
PATCH="$(cd "$(dirname "$0")" && pwd)/pixie-dust.patch"

rm -rf "$SRC" "$SRC.tar.gz"
wget -q "https://w1.fi/releases/hostapd-$VER.tar.gz" -O "$SRC.tar.gz"
tar xzf "$SRC.tar.gz" -C /tmp

# Apply the Pixie-Dust patch (zero the WPS E-S1/E-S2 secret nonces).
patch -p1 -d "$SRC" < "$PATCH"

cd "$SRC/hostapd"
cp defconfig .config
# hostapd ships WPS commented out; enable it so the AP answers external registrars.
sed -i 's/^#CONFIG_WPS=y/CONFIG_WPS=y/' .config

make -j"$(nproc)" hostapd
strip hostapd
cp hostapd "$DEST_DIR/hostapd-$GOARCH"
echo "built $DEST_DIR/hostapd-$GOARCH from hostapd $VER ($ARCH)"
