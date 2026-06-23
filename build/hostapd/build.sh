#!/usr/bin/env bash
# Tala WTE - Wireless Training Environment
# Copyright (c) 2026 VTEM Labs. All rights reserved.
#
# Builds the deliberately-weakened hostapd that Tala WTE embeds for opt-in
# vulnerable lab targets. It is stock hostapd 2.11 with WPS enabled and two
# source changes (see pixie-dust.patch):
#
#   1. Pixie-Dust:  the WPS enrollee/registrar secret nonces (E-S1/E-S2) are
#      forced to zero, so pixiewps recovers the WPS PIN offline in seconds.
#   2. PMKID:       the AP includes the RSN PMKID KDE in EAPOL msg 1/4 for
#      WPA2-PSK (stock hostapd deliberately omits it), so a clientless PMKID
#      capture (hcxdumptool -> hashcat 22000) works.
#
# Stock hostapd does neither, so a non-downgraded network is not vulnerable to
# either attack. This build is used ONLY for networks that opt in (the
# Pixie-Dust Downgrade or PMKID Exposed toggles); every other network uses
# system hostapd unchanged.
#
# Run it once on each target architecture: an x86_64 box produces hostapd-amd64,
# an aarch64 box produces hostapd-arm64. Output lands in internal/sim/pixie/,
# embedded into the Tala WTE binary via go:embed. Do NOT build on the Pi - build
# on a Proxmox amd64 VM and a Parallels arm64 VM.
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

rm -rf "$SRC" "$SRC.tar.gz"
wget -q "https://w1.fi/releases/hostapd-$VER.tar.gz" -O "$SRC.tar.gz"
tar xzf "$SRC.tar.gz" -C /tmp

# 1. Pixie-Dust: zero the WPS E-S1/E-S2 secret nonces.
sed -i 's/random_get_bytes(wps->snonce, 2 \* WPS_SECRET_NONCE_LEN)/(os_memset(wps->snonce, 0, 2 * WPS_SECRET_NONCE_LEN), 0)/' \
  "$SRC/src/wps/wps_enrollee.c" "$SRC/src/wps/wps_registrar.c"

# 2. PMKID: allow WPA2-PSK into the msg 1/4 PMKID path (PSK then falls through to
#    the existing rsn_pmkid() computation in that block).
sed -i 's@(wpa_key_mgmt_wpa_ieee8021x(sm->wpa_key_mgmt) ||@(wpa_key_mgmt_wpa_psk(sm->wpa_key_mgmt) ||\n\t     wpa_key_mgmt_wpa_ieee8021x(sm->wpa_key_mgmt) ||@' \
  "$SRC/src/ap/wpa_auth.c"

cd "$SRC/hostapd"
cp defconfig .config
# hostapd ships WPS commented out; enable it so the AP answers external registrars.
sed -i 's/^#CONFIG_WPS=y/CONFIG_WPS=y/' .config

make -j"$(nproc)" hostapd
strip hostapd
cp hostapd "$DEST_DIR/hostapd-$GOARCH"
echo "built $DEST_DIR/hostapd-$GOARCH from hostapd $VER ($ARCH)"
