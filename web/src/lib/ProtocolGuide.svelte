<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government
  use require a license from VTEM Labs. The Software may not be copied or
  redistributed. See the LICENSE file.
-->
<script lang="ts">
  interface Props {
    protocol: string;
    collapsed?: boolean;
    onToggle?: () => void;
  }

  let { protocol, collapsed = false, onToggle }: Props = $props();

  type GuideContent = {
    title: string;
    badge: string;
    overview: string;
    provides: string[];
    doesNotProvide: string[];
    vulns: { title: string; desc: string; cve?: string }[];
    tools: string[];
    useCases: string[];
  };

  const guides: Record<string, GuideContent> = {
    open: {
      title: 'Open Network',
      badge: 'badge-open',
      overview:
        'No authentication, no encryption. Any device can connect and all traffic is transmitted in cleartext over the air.',
      provides: ['Network connectivity'],
      doesNotProvide: ['Confidentiality', 'Integrity', 'Authentication', 'Replay protection'],
      vulns: [
        {
          title: 'Passive Eavesdropping',
          desc: 'All traffic visible to any device within radio range using a monitor mode adapter.'
        },
        {
          title: 'ARP Poisoning / MITM',
          desc: 'On the shared L2 segment, attackers can poison ARP tables to intercept traffic.'
        },
        {
          title: 'Rogue AP (Evil Twin)',
          desc: 'Without any client-side authentication, a cloned SSID is indistinguishable from the real AP.'
        },
        {
          title: 'DNS Spoofing',
          desc: 'Without DNSSEC, DNS responses can be forged to redirect clients to malicious hosts.'
        }
      ],
      tools: ['Wireshark', 'tcpdump', 'bettercap', 'arpspoof', 'mitmproxy'],
      useCases: [
        'Captive portal environments',
        'Testing devices without WPA support',
        'Simulating hotel/airport/coffee shop hotspots'
      ]
    },
    wep: {
      title: 'WEP',
      badge: 'badge-wpa',
      overview:
        'Wired Equivalent Privacy (1997). RC4 stream cipher with a 24-bit IV. Cryptographically broken since 2001 - the key is recoverable in minutes regardless of length. Present for training and tool demonstration only.',
      provides: ['Obfuscation only - no meaningful security'],
      doesNotProvide: [
        'Confidentiality',
        'Integrity (CRC-32 is forgeable)',
        'Replay protection',
        'Key management'
      ],
      vulns: [
        {
          title: 'FMS Attack (2001)',
          desc: 'Statistical key recovery from weak IVs in the RC4 keystream.'
        },
        {
          title: 'KoreK / PTW',
          desc: 'Recover the key from ~20k-80k captured IVs in minutes with aircrack-ng.'
        },
        {
          title: 'ARP Replay Injection',
          desc: 'Replay captured ARP frames to force rapid IV generation and accelerate the crack.'
        },
        {
          title: 'ChopChop / Fragmentation',
          desc: 'Recover keystream without the key to forge and inject packets.'
        },
        {
          title: 'Caffe Latte / Hirte',
          desc: 'Recover the key from an isolated client with no AP present.'
        }
      ],
      tools: ['airodump-ng', 'aireplay-ng', 'aircrack-ng', 'besside-ng', 'wifite'],
      useCases: [
        'WEP cracking demonstration',
        'aircrack-ng / PTW training',
        'Showing why legacy crypto must be retired'
      ]
    },
    wpa: {
      title: 'WPA (TKIP)',
      badge: 'badge-wpa',
      overview:
        'Transitional standard (2003). Uses TKIP over RC4 as a software fix for WEP while 802.11i was finalized. Considered legacy - avoid in production.',
      provides: [
        'Basic confidentiality via TKIP per-packet key mixing',
        'Integrity via Michael MIC'
      ],
      doesNotProvide: [
        'Forward secrecy',
        'Strong integrity (Michael MIC is broken)',
        'Protection against KRACK'
      ],
      vulns: [
        {
          title: 'Beck-Tews / Ohigashi-Morii (TKIP)',
          desc: 'Partial plaintext recovery of short TKIP packets; limited injection possible against QoS channels.'
        },
        {
          title: 'KRACK - Key Reinstallation',
          desc: 'Nonce reuse under TKIP leads to keystream recovery - more severe than in WPA2.',
          cve: 'CVE-2017-13077'
        },
        {
          title: '4-Way Handshake Dictionary Attack',
          desc: 'Capture EAPOL with airodump-ng, deauth to force reconnect, crack offline.'
        },
        {
          title: 'PMKID Attack',
          desc: 'Extract PMKID from first EAPOL frame - no active client required.'
        }
      ],
      tools: ['aircrack-ng', 'hashcat -m 22000', 'hcxdumptool', 'hcxtools', 'aireplay-ng'],
      useCases: ['Legacy device compatibility testing', 'Demonstrating TKIP weaknesses']
    },
    wpa2: {
      title: 'WPA2 (CCMP/AES)',
      badge: 'badge-wpa2',
      overview:
        'Current mainstream standard (802.11i, 2004). Mandates CCMP (AES-128 in CCM mode) providing strong confidentiality and integrity.',
      provides: [
        'Strong confidentiality (AES-CCMP)',
        'Replay protection via PN counter',
        'Frame integrity (CBC-MAC)'
      ],
      doesNotProvide: [
        'Forward secrecy',
        'Protection against offline dictionary attacks',
        'Mandatory PMF (optional)'
      ],
      vulns: [
        {
          title: 'PMKID Attack (2018)',
          desc: 'PMKID extracted from single EAPOL frame - no client association required. Crack offline with hashcat.'
        },
        {
          title: '4-Way Handshake Offline Crack',
          desc: 'Capture handshake, crack passphrase offline with hashcat -m 22000 + wordlist.'
        },
        {
          title: 'KRACK',
          desc: 'Key reinstallation via crafted retransmitted handshake messages.',
          cve: 'CVE-2017-13077'
        },
        {
          title: 'FragAttacks',
          desc: 'Frame aggregation/fragmentation vulnerabilities affecting most Wi-Fi devices.',
          cve: 'CVE-2020-26139'
        },
        {
          title: 'Deauth + Reconnect',
          desc: '802.11 deauth frames unauthenticated without PMF - force client reconnects to capture handshakes.'
        }
      ],
      tools: ['aircrack-ng', 'hashcat -m 22000', 'hcxdumptool', 'hcxtools', 'aireplay-ng -0'],
      useCases: [
        'Most common real-world network type',
        'PSK dictionary attack demonstration',
        'PMKID attack lab'
      ]
    },
    wps: {
      title: 'WPS',
      badge: 'badge-wps',
      overview:
        'Wi-Fi Protected Setup (2006). Designed to simplify AP pairing - critically flawed by design. PIN validation is split into two independent 4-digit halves, reducing complexity from 10^8 to 11,000.',
      provides: ['Easy device pairing for non-technical users'],
      doesNotProvide: ['Security - the PIN design is fundamentally broken'],
      vulns: [
        {
          title: 'Online PIN Brute Force',
          desc: 'Rate limiting was optional. Reaver or bully cycle all 11,000 PIN combinations in minutes to hours. This is the attack the lab target is built for: it advertises a registrar PIN and does not lock out, so reaver/bully recover the PIN and the WPA passphrase.'
        },
        {
          title: 'Pixie Dust Attack',
          desc: 'Many chipsets (Ralink, Broadcom, Realtek pre-2014) used weak RNG for the WPS E-S1/E-S2 nonces, so the PIN is recoverable offline in seconds with pixiewps. This is a real-hardware flaw: the lab AP runs hostapd with a strong RNG, so Pixie Dust will not succeed against it. Use the online PIN brute force here.'
        },
        {
          title: 'PBC Race Condition',
          desc: 'Two-minute PBC window allows attacker to activate PBC on a rogue AP simultaneously.'
        }
      ],
      tools: ['reaver', 'bully', 'pixiewps', 'wash'],
      useCases: [
        'Demonstrating the WPS PIN attack surface',
        'Online PIN recovery with reaver/bully',
        'Legacy device enumeration'
      ]
    },
    wpa3: {
      title: 'WPA3 (SAE)',
      badge: 'badge-wpa3',
      overview:
        'Current strong standard (2018). Replaces PSK with SAE (Simultaneous Authentication of Equals / Dragonfly), providing forward secrecy and resistance to offline dictionary attacks.',
      provides: [
        'Forward secrecy (ephemeral PMK per session)',
        'Resistance to offline dictionary attacks',
        'Mandatory PMF (802.11w)'
      ],
      doesNotProvide: [
        'Protection against Dragonblood side-channel (pre-patch)',
        'Full protection in Transition Mode from downgrade'
      ],
      vulns: [
        {
          title: 'Dragonblood - Timing Side-Channel',
          desc: 'Variable-time PWE derivation leaks password bits. Fixed in updated implementations.',
          cve: 'CVE-2019-9494'
        },
        {
          title: 'SAE Confirm Bypass',
          desc: 'Malformed SAE Confirm before Commit causes auth bypass in some implementations.',
          cve: 'CVE-2019-9496'
        },
        {
          title: 'Downgrade in Transition Mode',
          desc: 'Attacker suppresses WPA3 IEs → client falls back to WPA2-PSK.'
        },
        {
          title: 'SAE Anti-Clogging DoS',
          desc: 'Spoofed SAE Commit frames exhaust AP state before cookie threshold triggers.'
        }
      ],
      tools: ['dragonslayer (PoC)', 'wpa_supplicant (test client)'],
      useCases: [
        'Testing WPA3-capable client devices',
        'Dragonblood side-channel demonstration',
        'Transition mode downgrade testing'
      ]
    },
    wpa3_transition: {
      title: 'WPA3-Transition',
      badge: 'badge-wpa3',
      overview:
        'Allows both WPA2-PSK and WPA3-SAE clients on the same SSID simultaneously. The AP advertises both RSN IE and RSNXE. PMF is optional (unlike WPA3-only where it is required).',
      provides: [
        'Interoperability between WPA2 and WPA3 clients',
        'Forward secrecy for SAE-capable clients'
      ],
      doesNotProvide: [
        'Full WPA3 security for WPA2 clients',
        'Protection against downgrade for WPA3 clients'
      ],
      vulns: [
        {
          title: 'WPA2 Downgrade',
          desc: 'WPA3 client can be pushed to WPA2-PSK via management frame manipulation, losing forward secrecy.'
        },
        {
          title: 'All WPA2 vulnerabilities',
          desc: 'WPA2 clients connecting via PSK are subject to all WPA2 weaknesses.'
        }
      ],
      tools: ['aircrack-ng', 'hashcat', 'wpa_supplicant'],
      useCases: ['Gradual WPA3 migration simulation', 'Mixed-client environment testing']
    },
    wpa2_enterprise: {
      title: 'WPA2-Enterprise',
      badge: 'badge-enterprise',
      overview:
        'Corporate standard - per-user credentials via RADIUS and EAP. Clients authenticate with username/password (PEAP/TTLS) or certificates (EAP-TLS) tunneled inside TLS. FreeRADIUS + OpenLDAP provide the backend in Tala WTE; you set the test client identity and password on the network form.',
      provides: [
        'Per-user authentication (no shared passphrase)',
        'Credential isolation between users',
        'EAP-TLS provides mutual certificate auth'
      ],
      doesNotProvide: [
        'Protection against rogue RADIUS if cert validation disabled',
        'Forward secrecy from PMK (until WPA3-Enterprise)'
      ],
      vulns: [
        {
          title: 'Rogue RADIUS (hostapd-wpe)',
          desc: 'Self-signed cert presented by rogue AP; clients without cert validation send MSCHAPv2 hashes.'
        },
        {
          title: 'MSCHAPv2 is Broken (2012)',
          desc: 'Any captured PEAP-MSCHAPv2 exchange reduces to three 56-bit DES brute forces - fully crackable.'
        },
        {
          title: 'GTC Downgrade',
          desc: 'Inside PEAP tunnel, attacker AP requests EAP-GTC to obtain credentials in cleartext.'
        },
        {
          title: 'Missing CRL/OCSP',
          desc: 'Revoked client certs may still authenticate if CRL checking is not enforced.'
        },
        {
          title: 'Blast-RADIUS',
          desc: 'RADIUS over UDP without Message-Authenticator is forgeable via an MD5 chosen-prefix collision, letting an on-path attacker turn an Access-Reject into an Access-Accept.',
          cve: 'CVE-2024-3596'
        }
      ],
      tools: ['eaphammer', 'hostapd-wpe', 'asleap', 'hashcat -m 5500 (MSCHAPv2)', 'eapol_test'],
      useCases: [
        'Corporate network simulation',
        'PEAP credential harvest demonstration',
        'EAP-TLS mutual auth testing',
        'LDAP user management training'
      ]
    },
    wpa3_enterprise: {
      title: 'WPA3-Enterprise',
      badge: 'badge-enterprise',
      overview:
        'Strongest wireless standard. Adds Suite-B-192 cryptography (GCMP-256, BIP-GMAC-256) and mandatory PMF on top of WPA2-Enterprise. Requires EAP-TLS with 384-bit ECC or RSA-3072+.',
      provides: [
        'Suite-B-192 crypto (GCMP-256)',
        'Mandatory PMF',
        'Mutual certificate authentication'
      ],
      doesNotProvide: ['Compatibility with most client devices without Suite-B support'],
      vulns: [
        {
          title: 'Certificate Infrastructure Attacks',
          desc: 'Weaknesses in the PKI (expired CRLs, misconfigured OCSP) can allow unauthorized access.'
        },
        {
          title: 'Client Misconfiguration',
          desc: 'Clients that disable server cert validation remain vulnerable to MITM.'
        }
      ],
      tools: ['eapol_test', 'wpa_supplicant (EAP-TLS)'],
      useCases: [
        'High-security enterprise simulation',
        'Suite-B compliance testing',
        'Certificate lifecycle training'
      ]
    }
  };

  const guide = $derived(guides[protocol] ?? guides['open']);
</script>

<aside class="guide-panel" class:collapsed>
  <div class="guide-header">
    {#if !collapsed}
      <span class="guide-panel-title">Protocol Guide</span>
    {/if}
    <button
      class="action-btn"
      onclick={onToggle}
      title={collapsed ? 'Expand guide' : 'Collapse guide'}
    >
      {collapsed ? '?' : '×'}
    </button>
  </div>

  {#if !collapsed && guide}
    <div class="guide-body fade-in">
      <div class="guide-section">
        <div style="display:flex;align-items:center;gap:0.5rem;margin-bottom:0.75rem">
          <span class="badge {guide.badge}">{guide.title}</span>
        </div>
        <p>{guide.overview}</p>
      </div>

      <div class="guide-section">
        <h3>✓ Provides</h3>
        {#each guide.provides as item}
          <p style="color:var(--color-green)">+ {item}</p>
        {/each}
      </div>

      <div class="guide-section">
        <h3>✗ Does Not Provide</h3>
        {#each guide.doesNotProvide as item}
          <p style="color:var(--text-muted)">− {item}</p>
        {/each}
      </div>

      <div class="guide-section">
        <h3>Known Vulnerability Classes</h3>
        <p style="margin-bottom:0.75rem;font-style:italic">
          Informational reference only. Use external tools for testing.
        </p>
        {#each guide.vulns as v}
          <div class="vuln-item">
            <div class="vuln-title">{v.title}</div>
            <div class="vuln-desc">{v.desc}</div>
            {#if v.cve}
              <div class="vuln-cve">{v.cve}</div>
            {/if}
          </div>
        {/each}
      </div>

      <div class="guide-section">
        <h3>External Testing Tools</h3>
        <p style="margin-bottom:0.5rem">Point these at this network from another device:</p>
        <div style="display:flex;flex-wrap:wrap;gap:0.4rem">
          {#each guide.tools as tool}
            <code
              style="font-size:var(--font-size-xs);padding:2px 6px;background:var(--bg-card);border:1px solid var(--border-primary);border-radius:4px"
              >{tool}</code
            >
          {/each}
        </div>
      </div>

      <div class="guide-section">
        <h3>Recommended Use Cases</h3>
        {#each guide.useCases as uc}
          <p>• {uc}</p>
        {/each}
      </div>
    </div>
  {/if}
</aside>

<style>
  .guide-panel-title {
    font-size: var(--font-size-xs);
    font-weight: 600;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.08em;
  }
</style>
