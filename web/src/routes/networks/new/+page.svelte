<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government
  use require a license from VTEM Labs. The Software may not be copied or
  redistributed. See the LICENSE file.
-->
<script lang="ts">
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { networks, system, portals } from '$lib/api';
  import ProtocolGuide from '$lib/ProtocolGuide.svelte';
  import PortalPreviewModal from '$lib/components/PortalPreviewModal.svelte';

  import type { WirelessInterface } from '$lib/types';

  const channelMap: Record<string, { ch: number; label: string }[]> = {
    '2.4': [
      { ch: 1, label: '1 (2412 MHz)' },
      { ch: 2, label: '2 (2417 MHz)' },
      { ch: 3, label: '3 (2422 MHz)' },
      { ch: 4, label: '4 (2427 MHz)' },
      { ch: 5, label: '5 (2432 MHz)' },
      { ch: 6, label: '6 (2437 MHz)' },
      { ch: 7, label: '7 (2442 MHz)' },
      { ch: 8, label: '8 (2447 MHz)' },
      { ch: 9, label: '9 (2452 MHz)' },
      { ch: 10, label: '10 (2457 MHz)' },
      { ch: 11, label: '11 (2462 MHz)' }
    ],
    '5': [
      { ch: 36, label: '36 (5180 MHz)' },
      { ch: 40, label: '40 (5200 MHz)' },
      { ch: 44, label: '44 (5220 MHz)' },
      { ch: 48, label: '48 (5240 MHz)' },
      { ch: 52, label: '52 (5260 MHz) DFS' },
      { ch: 56, label: '56 (5280 MHz) DFS' },
      { ch: 60, label: '60 (5300 MHz) DFS' },
      { ch: 64, label: '64 (5320 MHz) DFS' },
      { ch: 100, label: '100 (5500 MHz) DFS' },
      { ch: 104, label: '104 (5520 MHz) DFS' },
      { ch: 108, label: '108 (5540 MHz) DFS' },
      { ch: 112, label: '112 (5560 MHz) DFS' },
      { ch: 116, label: '116 (5580 MHz) DFS' },
      { ch: 120, label: '120 (5600 MHz) DFS' },
      { ch: 124, label: '124 (5620 MHz) DFS' },
      { ch: 128, label: '128 (5640 MHz) DFS' },
      { ch: 132, label: '132 (5660 MHz) DFS' },
      { ch: 136, label: '136 (5680 MHz) DFS' },
      { ch: 140, label: '140 (5700 MHz) DFS' },
      { ch: 144, label: '144 (5720 MHz) DFS' },
      { ch: 149, label: '149 (5745 MHz)' },
      { ch: 153, label: '153 (5765 MHz)' },
      { ch: 157, label: '157 (5785 MHz)' },
      { ch: 161, label: '161 (5805 MHz)' },
      { ch: 165, label: '165 (5825 MHz)' }
    ],
    '6': [
      { ch: 1, label: '1 (5955 MHz)' },
      { ch: 5, label: '5 (5975 MHz)' },
      { ch: 9, label: '9 (5995 MHz)' },
      { ch: 13, label: '13 (6015 MHz)' },
      { ch: 17, label: '17 (6035 MHz)' },
      { ch: 21, label: '21 (6055 MHz)' },
      { ch: 25, label: '25 (6075 MHz)' },
      { ch: 29, label: '29 (6095 MHz)' },
      { ch: 33, label: '33 (6115 MHz)' },
      { ch: 37, label: '37 (6135 MHz)' },
      { ch: 41, label: '41 (6155 MHz)' },
      { ch: 45, label: '45 (6175 MHz)' },
      { ch: 49, label: '49 (6195 MHz)' },
      { ch: 53, label: '53 (6215 MHz)' },
      { ch: 57, label: '57 (6235 MHz)' },
      { ch: 61, label: '61 (6255 MHz)' },
      { ch: 65, label: '65 (6275 MHz)' },
      { ch: 69, label: '69 (6295 MHz)' },
      { ch: 73, label: '73 (6315 MHz)' },
      { ch: 77, label: '77 (6335 MHz)' },
      { ch: 81, label: '81 (6355 MHz)' },
      { ch: 85, label: '85 (6375 MHz)' },
      { ch: 89, label: '89 (6395 MHz)' },
      { ch: 93, label: '93 (6415 MHz)' }
    ]
  };

  const defaultChannel: Record<string, number> = { '2.4': 6, '5': 36, '6': 1 };

  let ssid = $state('');
  let protocol = $state('wpa2');
  let band = $state('2.4');
  let channel = $state(6);
  let passphrase = $state('');
  let iface = $state('');
  let clientIsolation = $state(false);
  let internetPassthrough = $state(true);
  let hidden = $state(false);
  let subnet = $state('10.0.0.0/24');
  let portalEnabled = $state(false);
  let selectedPortalId = $state('');
  let portalAuth = $state(false);

  let interfaces = $state<WirelessInterface[]>([]);
  let inUse = $state<Record<string, string>>({});
  let portalsList = $state<Record<string, any>[]>([]);
  let saving = $state(false);
  let error = $state('');

  const selectedPortalHTML = $derived(
    portalsList.find((p) => p.id === selectedPortalId)?.html ?? ''
  );
  const selectedPortalName = $derived(
    portalsList.find((p) => p.id === selectedPortalId)?.name ?? ''
  );
  let previewModalOpen = $state(false);

  const availableChannels = $derived(channelMap[band] ?? channelMap['2.4']);
  const needsPassphrase = $derived(
    ['wpa', 'wpa2', 'wps', 'wpa3', 'wpa3_transition', 'wep'].includes(protocol)
  );
  const isWEP = $derived(protocol === 'wep');
  const canHavePortal = $derived(protocol === 'open');

  // Accept any input and fit it to a valid WEP length (5/13 ASCII or 10/26 hex) rather than reject it.
  function normalizeWEPKey(input: string): { key: string; label: string } | null {
    const s = input ?? '';
    if (s.length === 0) return null;
    const isHex = /^[0-9a-fA-F]+$/.test(s);
    if (isHex && s.length === 10) return { key: s.toLowerCase(), label: '40-bit hex' };
    if (isHex && s.length === 26) return { key: s.toLowerCase(), label: '104-bit hex' };
    if (s.length === 5) return { key: s, label: '40-bit ASCII' };
    if (s.length === 13) return { key: s, label: '104-bit ASCII' };
    // Deterministic fit to a 13-char ASCII key: truncate if long, repeat-pad if short.
    let k = s;
    if (k.length > 13) k = k.slice(0, 13);
    else while (k.length < 13) k += s;
    k = k.slice(0, 13);
    return { key: k, label: '104-bit ASCII, fitted from your input' };
  }

  const wepEffective = $derived(isWEP ? normalizeWEPKey(passphrase) : null);

  // ap_bands can be narrower than radio bands (a chip may tune a band but not beacon on it); empty means unknown adapter, so don't restrict.
  const selectedAdapter = $derived(interfaces.find((i) => i.interface === iface));
  const apBands = $derived(
    selectedAdapter?.ap_bands?.length ? selectedAdapter.ap_bands : (selectedAdapter?.bands ?? [])
  );
  const bandLabel: Record<string, string> = { '2.4': '2.4 GHz', '5': '5 GHz', '6': '6 GHz' };
  function bandSupported(b: string): boolean {
    if (!apBands.length) return true; // unknown adapter: allow, server validates
    return apBands.includes(bandLabel[b]);
  }

  // Fall back to a host-capable band if the chosen adapter can't host the selected one (converges, no loop).
  $effect(() => {
    if (apBands.length && !bandSupported(band)) {
      const fallback = ['2.4', '5', '6'].find((b) => bandSupported(b));
      if (fallback && fallback !== band) {
        band = fallback;
        channel = defaultChannel[band] ?? 6;
      }
    }
  });

  function formatIfaceLabel(i: WirelessInterface): string {
    const parts = [i.interface];
    if (i.manufacturer && i.device_model) {
      parts.push(`${i.manufacturer} ${i.device_model}`);
    } else if (i.driver) {
      parts.push(i.driver);
    }
    return parts.join(' - ');
  }

  function onBandChange() {
    channel = defaultChannel[band] ?? 6;
  }

  onMount(async () => {
    try {
      const res = await system.interfaces();
      interfaces = res.interfaces ?? [];
      inUse = res.in_use ?? {};
      if (interfaces.length) {
        iface = interfaces[0].interface ?? '';
      }
    } catch {
      error = 'Failed to load wireless interfaces - check server connection';
    }
    try {
      portalsList = await portals.list();
      if (portalsList.length) selectedPortalId = portalsList[0].id;
    } catch {
      // Portals are optional, form still works without them
    }
    try {
      const st = await system.getSettings();
      if (st.ap_subnet) subnet = st.ap_subnet;
    } catch {
      // fall back to the default subnet already set
    }
  });

  async function save() {
    error = '';

    if (!ssid.trim()) {
      error = 'SSID Name is required';
      window.scrollTo(0, 0);
      return;
    }
    if (new TextEncoder().encode(ssid).length > 32) {
      error = 'SSID must be 32 bytes or fewer';
      window.scrollTo(0, 0);
      return;
    }
    if (!iface.trim()) {
      error = 'Wireless interface is required';
      window.scrollTo(0, 0);
      return;
    }
    if (isWEP) {
      if (!normalizeWEPKey(passphrase)) {
        error = 'WEP key is required';
        window.scrollTo(0, 0);
        return;
      }
    } else if (needsPassphrase) {
      if (passphrase.length < 8) {
        error = 'Passphrase must be at least 8 characters';
        window.scrollTo(0, 0);
        return;
      }
      if (passphrase.length > 63) {
        error = 'Passphrase must be 63 characters or fewer';
        window.scrollTo(0, 0);
        return;
      }
    }

    saving = true;
    try {
      const effPassphrase = isWEP ? (normalizeWEPKey(passphrase)?.key ?? '') : passphrase;
      const rec = await networks.create({
        ssid,
        protocol,
        band,
        channel,
        passphrase: needsPassphrase ? effPassphrase : '',
        interface: iface,
        client_isolation: clientIsolation,
        internet_passthrough: internetPassthrough,
        hidden,
        subnet,
        portal_enabled: canHavePortal ? portalEnabled : false,
        portal_html: canHavePortal && portalEnabled ? selectedPortalHTML : '',
        portal_auth: canHavePortal && portalEnabled ? portalAuth : false
      });
      goto(`/networks/${rec.id}`);
    } catch (e: any) {
      error = e?.message ?? 'Failed to create network';
      window.scrollTo(0, 0);
    }
    saving = false;
  }
</script>

<svelte:head><title>New Network - Tala WTE</title></svelte:head>

<div class="page-header">
  <div>
    <h1 class="page-title">New Network</h1>
    <p class="page-subtitle">Configure a wireless access point</p>
  </div>
  <div class="header-actions">
    <button class="btn btn-primary" onclick={save} disabled={saving}>
      {saving ? 'Creating...' : 'Create Network'}
    </button>
    <a href="/networks" class="btn">Cancel</a>
  </div>
</div>

{#if error}
  <div class="error-toast">
    <span>{error}</span>
    <button class="action-btn" onclick={() => (error = '')}>×</button>
  </div>
{/if}

<div class="detail-layout">
  <div class="config-main stack">
    <section class="panel">
      <div class="panel-head"><h2 class="panel-title">Network Profile</h2></div>
      <div class="panel-body">
        <div class="form-group" style="margin-bottom:var(--space-lg)">
          <label class="field-label" for="ssid">SSID Name *</label>
          <input
            class="input"
            id="ssid"
            bind:value={ssid}
            placeholder="e.g. TalaWTE-Target"
            maxlength="32"
          />
        </div>

        <div class="form-group" style="margin-bottom:{needsPassphrase ? 'var(--space-lg)' : '0'}">
          <label class="field-label" for="protocol">Security Protocol</label>
          <select class="input" id="protocol" bind:value={protocol}>
            <option value="open">Open (No Auth)</option>
            <option value="wep">WEP (Insecure - Legacy)</option>
            <option value="wpa">WPA (TKIP - Legacy)</option>
            <option value="wpa2">WPA2-Personal (AES)</option>
            <option value="wps">WPA2 + WPS</option>
            <option value="wpa3">WPA3-Personal (SAE)</option>
            <option value="wpa3_transition">WPA3-Transition (SAE+PSK)</option>
            <option value="wpa2_enterprise">WPA2-Enterprise (802.1X)</option>
            <option value="wpa3_enterprise">WPA3-Enterprise (Suite-B)</option>
          </select>
        </div>

        {#if needsPassphrase}
          <div class="form-group">
            <label class="field-label" for="passphrase"
              >{isWEP ? 'WEP Key' : 'Passphrase (min. 8 chars)'}</label
            >
            <input
              class="input"
              id="passphrase"
              type="text"
              bind:value={passphrase}
              placeholder={isWEP ? 'any text, ASCII, or hex' : 'Network passphrase'}
            />
            {#if isWEP && wepEffective}
              <p class="field-desc">
                Effective key ({wepEffective.label}):
                <code style="color:var(--accent)">{wepEffective.key}</code> - enter this exact value on
                test clients.
              </p>
            {:else if isWEP}
              <p class="field-desc">
                WEP keys are 5 or 13 ASCII chars or 10/26 hex. Any other input is fitted to a valid
                13-char key automatically.
              </p>
            {/if}
          </div>
        {/if}
      </div>
    </section>

    <section class="panel">
      <div class="panel-head"><h2 class="panel-title">Hardware</h2></div>
      <div class="panel-body">
        <div class="form-grid" style="margin-bottom:var(--space-lg)">
          <div class="form-group">
            <label class="field-label" for="band">Frequency Band</label>
            <select class="input" id="band" bind:value={band} onchange={onBandChange}>
              <option value="2.4" disabled={!bandSupported('2.4')}
                >2.4 GHz{bandSupported('2.4') ? '' : ' - not supported by this adapter'}</option
              >
              <option value="5" disabled={!bandSupported('5')}
                >5 GHz{bandSupported('5') ? '' : ' - not supported by this adapter'}</option
              >
              <option value="6" disabled={!bandSupported('6')}
                >6 GHz (Wi-Fi 6E){bandSupported('6')
                  ? ''
                  : ' - not supported by this adapter'}</option
              >
            </select>
            {#if selectedAdapter && apBands.length && apBands.length < 3}
              <div class="field-desc" style="color:var(--text-muted)">
                {selectedAdapter.chipset ?? 'This adapter'} can host an AP on: {apBands.join(', ')}.
              </div>
            {/if}
          </div>
          <div class="form-group">
            <label class="field-label" for="channel">Channel</label>
            <select class="input" id="channel" bind:value={channel}>
              {#each availableChannels as c}
                <option value={c.ch}>{c.label}</option>
              {/each}
            </select>
          </div>
        </div>

        <div class="form-group">
          <label class="field-label" for="iface">Wireless Interface</label>
          {#if interfaces.length}
            <select class="input" id="iface" bind:value={iface}>
              {#each interfaces as i}
                <option value={i.interface}>{formatIfaceLabel(i)}</option>
              {/each}
            </select>
            {#if Object.keys(inUse).length}
              <div class="field-desc" style="color:var(--text-muted)">
                In use by running networks (run a concurrent network on a free adapter): {Object.entries(
                  inUse
                )
                  .map(([i, s]) => `${i} -> ${s}`)
                  .join(', ')}
              </div>
            {/if}
          {:else}
            <input class="input" id="iface" bind:value={iface} placeholder="e.g. wlan0" />
            <span class="field-desc">No interfaces detected - enter manually</span>
          {/if}
        </div>
      </div>
    </section>

    <section class="panel">
      <div class="panel-head"><h2 class="panel-title">Topology</h2></div>
      <div class="panel-body stack">
        <div class="toggle-field">
          <div>
            <div class="toggle-name">Internet Passthrough</div>
            <div class="field-desc">NAT traffic from connected clients to uplink</div>
          </div>
          <input type="checkbox" bind:checked={internetPassthrough} />
        </div>

        <div class="form-group">
          <label class="field-label" for="subnet">Network Subnet</label>
          <input class="input" id="subnet" bind:value={subnet} placeholder="10.0.0.0/24" />
          <span class="field-desc">
            CIDR for the LAN clients join (gateway <code>.1</code>, DHCP <code>.10</code>-<code
              >.250</code
            >). Defaults from Settings.
          </span>
        </div>

        <div class="toggle-field">
          <div>
            <div class="toggle-name">Client Isolation</div>
            <div class="field-desc">Prevent clients from communicating with each other</div>
          </div>
          <input type="checkbox" bind:checked={clientIsolation} />
        </div>

        <div class="toggle-field">
          <div>
            <div class="toggle-name">Hidden Network</div>
            <div class="field-desc">
              Do not broadcast the SSID in beacons; clients must enter the name to connect
            </div>
          </div>
          <input type="checkbox" bind:checked={hidden} />
        </div>

        {#if canHavePortal}
          <div class="toggle-field">
            <div>
              <div class="toggle-name">Captive Portal Sandbox</div>
              <div class="field-desc">
                Intercept unauthenticated traffic and serve a portal page
              </div>
            </div>
            <input type="checkbox" bind:checked={portalEnabled} />
          </div>

          {#if portalEnabled}
            <div class="form-group portal-nest">
              <label class="field-label" for="portalSelect">Portal Module</label>
              <select class="input" id="portalSelect" bind:value={selectedPortalId}>
                {#if portalsList.length === 0}
                  <option value="" disabled>No portals available (Create in Captive Portals)</option
                  >
                {/if}
                {#each portalsList as p}
                  <option value={p.id}
                    >{p.name}{p.html?.startsWith('fs:') ? ' (bundle)' : ''}</option
                  >
                {/each}
              </select>
              {#if selectedPortalId}
                <div class="portal-preview">
                  <div class="portal-preview-head">
                    <span>Preview</span>
                    <button
                      type="button"
                      class="popout-btn"
                      onclick={() => (previewModalOpen = true)}
                    >
                      Pop out
                    </button>
                  </div>
                  <iframe
                    class="portal-preview-frame"
                    title="Portal preview"
                    src="/api/wte/portals/{selectedPortalId}/preview"
                    sandbox="allow-same-origin"
                    loading="lazy"
                  ></iframe>
                </div>
              {/if}
              <div class="toggle-field" style="margin-top:var(--space-sm)">
                <div>
                  <div class="toggle-name">Require Login (Directory / LDAP)</div>
                  <div class="field-desc">
                    Validate submitted username and password against the directory before granting
                    access, like a corporate or ISP hotspot. Failed logins are denied and recorded.
                  </div>
                </div>
                <input type="checkbox" bind:checked={portalAuth} />
              </div>
            </div>
          {/if}
        {/if}
      </div>
    </section>
  </div>

  <ProtocolGuide {protocol} />
</div>

<PortalPreviewModal
  bind:open={previewModalOpen}
  portalId={selectedPortalId}
  portalName={selectedPortalName}
/>

<style>
  .config-main {
    flex: 1;
    min-width: 0;
  }

  .toggle-name {
    font-size: var(--font-size-sm);
    font-weight: 500;
    color: var(--text-primary);
  }

  .toggle-field .field-desc {
    margin-top: 2px;
  }

  .portal-nest {
    gap: var(--space-sm);
    margin-left: var(--space-md);
    padding-left: var(--space-lg);
    border-left: 2px solid var(--border-primary);
  }
  .portal-preview {
    margin-top: var(--space-sm);
    border: 1px solid var(--border-primary);
    border-radius: var(--radius-sm);
    overflow: hidden;
  }
  .portal-preview-head {
    font-size: var(--font-size-xs);
    color: var(--text-muted);
    padding: 4px 10px;
    background: var(--bg-input);
    border-bottom: 1px solid var(--border-primary);
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .popout-btn {
    background: none;
    border: 1px solid var(--border-primary);
    border-radius: var(--radius-sm);
    color: var(--text-secondary);
    font-size: var(--font-size-xs);
    padding: 2px 8px;
    cursor: pointer;
  }
  .popout-btn:hover {
    color: var(--text-primary);
    border-color: var(--color-cyan);
  }
  .portal-preview-frame {
    width: 100%;
    height: 340px;
    border: 0;
    display: block;
    background: #fff;
  }
</style>
