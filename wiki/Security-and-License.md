# Security and License

Tala WTE is a vulnerable-by-design wireless training range. This page covers what that means for safe operation, how to deploy it without putting anything at risk, and the terms under which you are allowed to use it.

## Vulnerable by design

Tala WTE deliberately stands up insecure networks and weak credentials so you have something realistic to practice against. That is the whole point: WEP, legacy WPA/TKIP, captive portals that harvest cleartext logins, traffic generators that replay passwords in the clear, and a directory full of crackable accounts. None of it is hardened, and none of it is meant to be.

Treat the entire box as untrusted lab equipment:

- It broadcasts real, attackable access points on real radios.
- It records everything submitted to a captive portal, including credentials, the device MAC, IP, and browser.
- Its traffic generators send logins over the wire in cleartext on purpose so they can be captured and cracked.
- Its LDAP directory ships a realistic password mix (some weak, some semi-personal, some strong) so cracking exercises feel real.

Run it only in an isolated lab network. Never run it on a production or corporate LAN, and never expose it to the internet or to untrusted clients. See [[Installation]] for the recommended setup.

## Deployment and isolation guidance

From the README deployment guidance and the way the binary is built:

- The web console is served over HTTPS on port 8443. It (and the directory, RADIUS, and portal services) should not be reachable from the internet or from untrusted networks. Tala WTE needs an outbound internet uplink for client traffic generation and updates, but it does not need, and should not have, inbound exposure.
- Port 80 exists only to redirect to HTTPS on 8443. The LDAP directory listens on 127.0.0.1:3389 and RADIUS on 1812; these are for the box's own enterprise auth chain, not for outside callers.
- Only transmit on Wi-Fi channels and a regulatory domain you are authorized to use. Set the correct country under [[Settings]] so hostapd advertises a legal domain and only offers channels you may broadcast on. The license also requires that your use be lawful and authorized on any network you operate it against.
- Put the box on its own segment with no path to anything you care about. Because internet passthrough NATs client traffic out the uplink, anything a connected client or pack member does reaches the internet through that uplink. Turn Internet Passthrough off on a network to keep an exercise sealed.

## Admin account and first run

Tala WTE never auto-provisions an administrator and never prints credentials to a terminal. On a fresh install the console shows a setup screen, and you create the single administrator account in the browser. You must acknowledge the license before the account can be created. See [[Installation]] and [[Troubleshooting]].

There is no default login and no shared password baked into the build.

## License summary

Tala WTE is free for personal and non-profit use. This is a summary; the controlling text is the full [LICENSE](https://github.com/vtemlabs/tala-wte/blob/main/LICENSE) file. The full license text is also available on its own page: [Licensing](/licensing/).

### What is free

- Use by an individual for their own private learning, experimentation, or skills practice.
- Use by or on behalf of a bona fide non-profit or not-for-profit school, university, institution, or organization for non-commercial educational purposes, where the software is not used to generate profit or revenue.

### Non-profit events

Non-profit use at an event (for example, a non-profit CTF, class, workshop, competition, or conference) is permitted only if you:

1. obtain prior written approval from VTEM Labs;
2. prominently display the VTEM Labs logo and the words "Powered by VTEM Labs" at the event; and
3. link to vtemlabs.com.

### What requires a license

Commercial or for-profit use requires a separate license from VTEM Labs (not just approval). This includes, among other things:

- Any use by, for, or on behalf of a for-profit business, employer, client, school, training provider, bootcamp, certification program, company, agency, institution, or organization.
- Any use by, for, or on behalf of any government or government agency.
- Paid training, paid Capture-the-Flag projects, and any use that generates or is intended to generate revenue, directly or indirectly.
- Standing up a for-profit wireless penetration testing course (or similar offering) and using Tala WTE, or any variant or copy of it, as the infrastructure or training material.

Important: a CTF or activity held at, or as part of, a conference, convention, or venue that charges paid admission or registration is For-Profit Use and requires a commercial license, regardless of the non-profit status of the event or its organizer.

### Other restrictions

Without written authorization and a license, you may not rebrand the software, remove or alter its notices or branding, claim authorship of it, or copy, redistribute, mirror, or otherwise share it with third parties. All rights not expressly granted are reserved by VTEM Labs.

### Licensing can be non-monetary

A license is not always monetary. For conferences, events, and CTFs, VTEM Labs may grant a license in exchange for non-monetary consideration instead of a fee, for example making VTEM Labs a high-tier sponsor of the event with logo placement and brand recognition, and sharing the event's attendee, contact, and sponsor lists. Terms are agreed in writing with VTEM Labs.

### About VTEM Labs

VTEM Labs, Inc. is a self-funded Service-Disabled Veteran-Owned Small Business (SDVOSB). Support, licensing, and sponsorship are what fund the next release. Commercial licensing inquiries go through vtemlabs.com/contact.

## Disclaimer and lawful use

The software is provided "as is", without warranty of any kind, and VTEM Labs is not liable for any claim or damages arising from its use. Tala WTE is intended for authorized wireless security training and testing only. You are solely responsible for ensuring your use is lawful and authorized on any network or system you operate it against.

See also: [[Installation]], [[Settings]], [[Troubleshooting]].
