# Changelog

Notable changes to Tala WTE, newest first. These notes are written for the people
who use the tool, not generated from commit messages.

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
