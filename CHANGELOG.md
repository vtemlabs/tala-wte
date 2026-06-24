# Changelog

Notable changes to Tala WTE, newest first. These notes are written for the people
who use the tool, not generated from commit messages.

## v1.0.3

**OWE (Enhanced Open).** You can now stand up an **OWE** network. To a user it
looks like an open hotspot, no password to join, but every client negotiates its
own per-session key with the access point through an unauthenticated
Diffie-Hellman exchange (RFC 8110, marketed as Enhanced Open), so the casual
over-the-air sniffing that exposes a plain open network no longer works.
Protected Management Frames are mandatory, exactly as the standard requires. The
range teaches both sides: why OWE kills passive capture on "open" Wi-Fi, and why
it still authenticates nothing, so a rogue OWE or open access point with the same
name stays indistinguishable to a client. A separate **OWE-Transition** type
mirrors how Enhanced Open actually ships, one radio beaconing an open SSID
alongside a companion OWE BSS, so you can practise the transition-mode downgrade
end to end.

**WPA2 + 802.11r (Fast Transition).** WPA2-Personal is now joined by a **WPA2 +
802.11r** network type, the Fast BSS Transition amendment. The access point
advertises a mobility domain and pre-derives a PMK-R0/PMK-R1 key hierarchy so a
client roams between access points in milliseconds instead of repeating the full
four-way handshake each time. It changes roaming, not PSK strength, which is the
point: the range lets you study the FT key hierarchy and validate handshake
capture against an 802.11r access point.

**Enterprise authentication that just works.** The full 802.1X stack now
auto-provisions itself on first boot, the certificate authority, the RADIUS
server certificate, the directory, and the FreeRADIUS-to-LDAP wiring, so a
WPA2/WPA3-Enterprise SSID comes up with no manual setup step. The provisioning is
idempotent, so restarts and upgrades leave a working stack working.

**Hardening and reliability.** Captured-network identifiers are now validated and
the helper commands the server runs are hardened, closing an input-handling gap.
The WEP key path is sanitized, and hostapd's diagnostic logs are now per-instance
so several access points running at once no longer overwrite each other's logs.

**Documentation site.** The full Tala WTE documentation and wireless field manual
is now online at https://tala-wte.vtemlabs.com, with protocol-by-protocol
coverage, an attack catalog, the toolkit, and defensive guides for enterprise,
home, and travel security.

## v1.0.2

**WPS Pixie-Dust downgrade.** You can now make a WPS network vulnerable to the
Pixie-Dust attack on purpose. WPS networks already shipped a recoverable PIN for
the online brute force (reaver, bully); the new **Pixie-Dust Downgrade** toggle on
the network form (it appears once you pick WPA2 + WPS) also makes the access point
hand out predictable WPS nonces, so `pixiewps` recovers the PIN offline in seconds.
Leave it off and the network resists Pixie Dust the way a modern, patched router
does - so one range teaches both sides: why a current AP defeats Pixie Dust, and
how an old chipset falls to it in seconds. The patched access-point software is
built into Tala WTE; there is nothing extra to install.

**PMKID exposure (clientless capture).** WPA2-Personal networks have a new **PMKID
Exposed** toggle. Off (the default), the access point withholds the PMKID like a
modern router, so grabbing the passphrase needs a full four-way handshake from a
connected client. On, the AP advertises the RSN PMKID in the first handshake frame,
so the passphrase can be captured **clientlessly** - point `hcxdumptool` at it with
no client connected, then crack it with hashcat. One range now teaches both the
clientless PMKID shortcut and why a modern AP forces the harder handshake capture.
The same built-in patched access-point software handles it.
