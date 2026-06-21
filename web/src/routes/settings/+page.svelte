<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government
  use require a license from VTEM Labs. The Software may not be copied or
  redistributed. See the LICENSE file.
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { pb, system, type VersionStatus } from '$lib/api';
  import { toast } from '$lib/stores/toast';
  import HardwareCard from '$lib/HardwareCard.svelte';
  import LicenseModal from '$lib/components/LicenseModal.svelte';
  import GuideModal from '$lib/components/GuideModal.svelte';
  import { GUIDES } from '$lib/guides';

  import type { WirelessInterface } from '$lib/types';

  let showLicense = $state(false);
  let guideOpen = $state(false);

  // Software updates (current version + GitHub release check).
  let versionInfo = $state<VersionStatus | null>(null);
  let updating = $state(false);
  const displayVersion = $derived(versionInfo?.current ?? '0.1.0');

  let interfaces = $state<WirelessInterface[]>([]);
  // Adapters claimed by a running network. The main-namespace scan can't see them
  // (their phy is in the network's namespace), so the backend returns full
  // hardware detail cached at start-time, with an in_use_by field for the SSID.
  let inUseAdapters = $state<any[]>([]);
  let unsupported = $state<{ usb_id: string; usb_path?: string; name: string; reason: string }[]>(
    []
  );
  let healing = $state<Record<string, boolean>>({});

  // Free radios first, then in-use ones - all rendered as full hardware cards.
  const allAdapters = $derived([...interfaces, ...inUseAdapters]);
  let uplinkIface = $state('eth0');
  let countryCode = $state('US');
  let apSubnet = $state('10.0.0.0/24');
  let saving = $state(false);
  let saved = $state(false);

  // Instance role (AP vs client) and the one-button swap between them.
  let mode = $state<'ap' | 'client'>('ap');
  let swapping = $state(false);
  // Pack agent key (client mode): the control token a pack leader uses to drive this client.
  let agentKey = $state('');

  // The loaded value is folded in below so a region not on this list still shows.
  const COUNTRIES: { code: string; name: string }[] = [
    { code: 'US', name: 'United States' },
    { code: 'CA', name: 'Canada' },
    { code: 'GB', name: 'United Kingdom' },
    { code: 'IE', name: 'Ireland' },
    { code: 'DE', name: 'Germany' },
    { code: 'FR', name: 'France' },
    { code: 'NL', name: 'Netherlands' },
    { code: 'ES', name: 'Spain' },
    { code: 'IT', name: 'Italy' },
    { code: 'SE', name: 'Sweden' },
    { code: 'CH', name: 'Switzerland' },
    { code: 'JP', name: 'Japan' },
    { code: 'AU', name: 'Australia' },
    { code: 'NZ', name: 'New Zealand' },
    { code: 'BR', name: 'Brazil' },
    { code: 'ZA', name: 'South Africa' },
    { code: 'AE', name: 'United Arab Emirates' }
  ];

  const countryOptions = $derived(
    COUNTRIES.some((c) => c.code === countryCode)
      ? COUNTRIES
      : [{ code: countryCode, name: countryCode }, ...COUNTRIES]
  );

  async function loadInterfaces() {
    const ifaceRes = await system.interfaces();
    interfaces = ifaceRes.interfaces ?? [];
    inUseAdapters = (ifaceRes as any).in_use_adapters ?? [];
    unsupported = (ifaceRes as any).unsupported ?? [];
  }

  // heal runs a USB-reset recovery on a wedged adapter, then refreshes the list.
  async function heal(key: string, target: { interface?: string; usb_path?: string }) {
    healing = { ...healing, [key]: true };
    try {
      const res = await system.heal(target);
      toast.success(res.message || 'Adapter recovered');
      await loadInterfaces();
    } catch (e: any) {
      toast.err(e?.message ?? 'Heal failed');
    } finally {
      healing = { ...healing, [key]: false };
    }
  }

  onMount(async () => {
    try {
      const [ifaceRes, settingsRes] = await Promise.all([
        system.interfaces(),
        system.getSettings()
      ]);
      interfaces = ifaceRes.interfaces ?? [];
      inUseAdapters = (ifaceRes as any).in_use_adapters ?? [];
      unsupported = (ifaceRes as any).unsupported ?? [];
      if (settingsRes.uplink_iface) uplinkIface = settingsRes.uplink_iface;
      if (settingsRes.country_code) countryCode = settingsRes.country_code;
      if (settingsRes.ap_subnet) apSubnet = settingsRes.ap_subnet;
      const st = await fetch('/api/wte/system/status').then((r) => r.json());
      if (st?.mode) mode = st.mode;
      if (mode === 'client') {
        agentKey =
          (
            await fetch('/api/wte/client/agent-key', {
              headers: { Authorization: pb.authStore.token }
            }).then((r) => r.json())
          )?.key ?? '';
      }
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to load settings');
    }
    // The release check hits GitHub, so load it separately; a failure here must
    // not block the rest of the settings page.
    try {
      versionInfo = await system.version();
    } catch {
      versionInfo = null;
    }
  });

  async function applyUpdate() {
    if (!versionInfo?.update_available) return;
    if (
      !confirm(
        `Update Tala WTE from ${versionInfo.current} to ${versionInfo.latest}?\n\nThe service will restart and the console will briefly disconnect.`
      )
    ) {
      return;
    }
    updating = true;
    try {
      const res = await system.update();
      toast.success(res.message ?? 'Update installed; restarting');
      // The backend restarts the service ~2s after responding. Poll the version
      // endpoint until it answers on the new build, then reload the console.
      waitForRestart();
    } catch (e: any) {
      toast.err(e?.message ?? 'Update failed');
      updating = false;
    }
  }

  // Poll until the service is back, then reload so the new frontend is served.
  function waitForRestart() {
    let elapsed = 0;
    const tick = async () => {
      elapsed += 3;
      try {
        const v = await system.version();
        // Back up and reporting the new version: reload to pick up the new UI.
        if (v && (!versionInfo || v.current === versionInfo.latest || !v.update_available)) {
          window.location.reload();
          return;
        }
      } catch {
        // Service still bouncing; keep waiting.
      }
      if (elapsed < 120) {
        setTimeout(tick, 3000);
      } else {
        updating = false;
        toast.err('Service did not come back within 2 minutes; reload manually.');
      }
    };
    setTimeout(tick, 5000);
  }

  async function save() {
    saving = true;
    try {
      await system.saveSettings({
        uplink_iface: uplinkIface,
        country_code: countryCode,
        ap_subnet: apSubnet
      });
      saved = true;
      toast.success('Settings saved');
      setTimeout(() => (saved = false), 3000);
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to save settings');
    }
    saving = false;
  }

  async function swapMode() {
    const target = mode === 'client' ? 'ap' : 'client';
    const label = target === 'ap' ? 'Server (AP)' : 'Client';
    if (
      !confirm(
        `Switch this instance to ${label} mode?\n\nThe service installs ${label} dependencies and restarts. The console disconnects and reloads when it is back; this can take a minute.`
      )
    ) {
      return;
    }
    swapping = true;
    try {
      const r = await fetch('/api/wte/system/mode', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: pb.authStore.token },
        body: JSON.stringify({ mode: target })
      });
      if (!r.ok) throw new Error((await r.json())?.error ?? 'swap failed');
      toast.success(`Switching to ${label} mode; the console will reconnect`);
      pollForMode(target);
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to switch mode');
      swapping = false;
    }
  }

  async function regenKey() {
    if (
      !confirm(
        'Regenerate the agent key? Any pack leader using the old key loses access until you re-add this client.'
      )
    ) {
      return;
    }
    try {
      agentKey =
        (
          await (
            await fetch('/api/wte/client/agent-key/regenerate', {
              method: 'POST',
              headers: { Authorization: pb.authStore.token }
            })
          ).json()
        )?.key ?? agentKey;
      toast.success('Agent key regenerated');
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to regenerate key');
    }
  }
  async function copyKey() {
    try {
      await navigator.clipboard.writeText(agentKey);
      toast.success('Agent key copied');
    } catch {
      toast.err('Copy failed');
    }
  }

  // Poll status until the service is back in the target role, then reload.
  function pollForMode(target: string) {
    let elapsed = 0;
    const tick = async () => {
      elapsed += 3;
      try {
        const st = await fetch('/api/wte/system/status').then((r) => r.json());
        if (st?.mode === target) {
          window.location.href = '/';
          return;
        }
      } catch {
        // service still restarting; keep waiting
      }
      if (elapsed < 240) {
        setTimeout(tick, 3000);
      } else {
        swapping = false;
        toast.err('Mode did not switch within 4 minutes; reload manually.');
      }
    };
    setTimeout(tick, 5000);
  }
</script>

<svelte:head><title>Settings - Tala WTE</title></svelte:head>

<div class="page-header">
  <div>
    <h1 class="page-title">Settings</h1>
    <p class="page-subtitle">System configuration - regulatory domain, uplink, services</p>
  </div>
  <div class="header-actions">
    {#if saved}<span class="badge badge-success">Saved</span>{/if}
    <button class="btn" onclick={() => (guideOpen = true)}>Guide</button>
    <button class="btn btn-primary" onclick={save} disabled={saving}
      >{saving ? 'Saving...' : 'Save Changes'}</button
    >
  </div>
</div>

<GuideModal bind:open={guideOpen} title={GUIDES.settings.title} doc={GUIDES.settings.doc} />

<div class="grid grid-2" style="align-items:start">
  <div class="stack">
    <section class="panel">
      <div class="panel-head"><h2 class="panel-title">Instance Role</h2></div>
      <div class="panel-body">
        <div class="role-row">
          <div>
            <div class="role-now">{mode === 'client' ? 'Client' : 'Server (AP)'}</div>
            <span class="field-desc">
              {mode === 'client'
                ? 'This box joins a network and generates traffic.'
                : 'This box broadcasts networks as an access point.'}
            </span>
          </div>
          <button class="btn btn-primary" onclick={swapMode} disabled={swapping}>
            {swapping
              ? 'Switching...'
              : mode === 'client'
                ? 'Switch to Server mode'
                : 'Switch to Client mode'}
          </button>
        </div>
        <span class="field-desc">
          Switching installs the other role's dependencies and restarts the service; the console
          reconnects automatically.
        </span>
      </div>
    </section>

    {#if mode === 'client'}
      <section class="panel">
        <div class="panel-head"><h2 class="panel-title">Pack Agent Key</h2></div>
        <div class="panel-body">
          <span class="field-desc"
            >Add this client to a pack leader using its address and this key.</span
          >
          <div class="key-val">{agentKey || 'generating...'}</div>
          <div class="key-actions">
            <button class="btn btn-sm btn-secondary" onclick={copyKey}>Copy key</button>
            <button class="btn btn-sm" onclick={regenKey}>Regenerate</button>
          </div>
        </div>
      </section>
    {/if}

    <section class="panel">
      <div class="panel-head"><h2 class="panel-title">Radio &amp; Network</h2></div>
      <div class="panel-body">
        <div class="field">
          <label class="field-label" for="country">Regulatory Domain</label>
          <select class="input" id="country" bind:value={countryCode}>
            {#each countryOptions as c}
              <option value={c.code}>{c.code} - {c.name}</option>
            {/each}
          </select>
          <span class="field-desc">
            Sets the country hostapd advertises and applies it with <code>iw reg set</code>. This
            decides which channels are legal and whether 5 GHz / 6 GHz AP mode is allowed. The world
            domain blocks 5 GHz beaconing, so this must match where the box operates.
          </span>
        </div>

        <div class="field">
          <label class="field-label" for="uplinkIface">Uplink Interface (Internet)</label>
          <input
            class="input"
            id="uplinkIface"
            bind:value={uplinkIface}
            placeholder="e.g. eth0, wlan1"
          />
          <span class="field-desc"
            >The interface connected to the internet, used for NAT passthrough on networks that
            allow it.</span
          >
        </div>

        <div class="field">
          <label class="field-label" for="apSubnet">Default Network Subnet</label>
          <input class="input" id="apSubnet" bind:value={apSubnet} placeholder="10.0.0.0/24" />
          <span class="field-desc"
            >The default LAN/subnet (CIDR) handed to clients that join a network. The gateway is
            <code>.1</code> and DHCP serves <code>.10</code>-<code>.250</code>. Each network can
            override this when created.</span
          >
        </div>
      </div>
    </section>

    <section class="panel">
      <div class="panel-head">
        <h2 class="panel-title">Software Updates</h2>
        {#if versionInfo?.update_available}<span class="badge badge-success">Update available</span
          >{/if}
      </div>
      <div class="panel-body">
        <div class="meta-grid">
          <div class="meta-row">
            <div class="meta-key">Installed</div>
            <div class="meta-val">v{displayVersion}</div>
          </div>
          {#if versionInfo?.latest}
            <div class="meta-row">
              <div class="meta-key">Latest release</div>
              <div class="meta-val">v{versionInfo.latest}</div>
            </div>
          {/if}
        </div>

        {#if versionInfo?.is_dev}
          <p class="update-note dim">
            This is a local development build. In-place updates are disabled; install a released
            binary to enable them.
          </p>
        {:else if updating}
          <p class="update-note">
            Installing v{versionInfo?.latest}. The service is restarting and the console will
            reconnect automatically.
          </p>
        {:else if versionInfo?.update_available}
          <p class="update-note">
            Version v{versionInfo.latest} is available. Updating downloads the verified binary, replaces
            the running service, and restarts it.
          </p>
          <div class="update-actions">
            <button class="btn btn-primary btn-sm" onclick={applyUpdate} disabled={updating}
              >Update to v{versionInfo.latest}</button
            >
            {#if versionInfo.release_url}
              <a
                class="btn btn-sm"
                href={versionInfo.release_url}
                target="_blank"
                rel="noopener noreferrer">Release notes</a
              >
            {/if}
          </div>
        {:else if versionInfo?.error}
          <p class="update-note dim">Could not check for updates ({versionInfo.error}).</p>
        {:else if versionInfo}
          <p class="update-note dim">You are running the latest version.</p>
        {:else}
          <p class="update-note dim">Checking for updates...</p>
        {/if}
      </div>
    </section>
  </div>

  <LicenseModal bind:open={showLicense} />

  <div class="stack">
    <section class="panel">
      <div class="panel-head">
        <h2 class="panel-title">Wireless Interfaces</h2>
        {#if allAdapters.length}<span class="count-pill">{allAdapters.length}</span>{/if}
      </div>
      <div class="panel-body">
        {#if allAdapters.length}
          <div class="stack">
            {#each allAdapters as adapter}
              <HardwareCard {adapter} />
            {/each}
          </div>
        {/if}
        {#if unsupported.length}
          <div class="unsupported-list">
            {#each unsupported as u}
              <div class="unsupported-row">
                <div class="unsupported-info">
                  <div class="unsupported-name">{u.name}</div>
                  <div class="unsupported-reason">{u.reason}</div>
                </div>
                <button
                  class="btn btn-sm"
                  disabled={healing[u.usb_id]}
                  onclick={() => heal(u.usb_id, { usb_path: u.usb_path })}
                  >{healing[u.usb_id] ? 'Healing...' : 'Heal'}</button
                >
              </div>
            {/each}
          </div>
        {/if}
        {#if !allAdapters.length && !unsupported.length}
          <div class="empty-state" style="padding:var(--space-2xl)">
            <p>No wireless interfaces detected.</p>
          </div>
        {/if}
      </div>
    </section>

    <section class="panel">
      <div class="panel-head"><h2 class="panel-title">Services</h2></div>
      <div class="panel-body">
        <div class="meta-grid">
          <div class="meta-row">
            <div class="meta-key">PocketBase</div>
            <div class="meta-val">:8090 (embedded)</div>
          </div>
          <div class="meta-row">
            <div class="meta-key">FreeRADIUS</div>
            <div class="meta-val">:1812 / :1813</div>
          </div>
          <div class="meta-row">
            <div class="meta-key">OpenLDAP</div>
            <div class="meta-val">127.0.0.1:3389</div>
          </div>
          <div class="meta-row">
            <div class="meta-key">Portal Server</div>
            <div class="meta-val">:8080 (per-network)</div>
          </div>
        </div>
      </div>
    </section>

    <section class="panel">
      <div class="panel-head"><h2 class="panel-title">About &amp; License</h2></div>
      <div class="panel-body">
        <p class="about-line">
          Tala WTE v{displayVersion} - a VTEM Labs Wireless Training Environment.
        </p>
        <p class="about-line dim">
          &copy; 2026 VTEM Labs. Free for personal and non-profit use. Commercial and for-profit
          use, including paid training, paid CTF, and use by any for-profit school, institution,
          company, government, or government agency, requires written authorization and a license
          from VTEM Labs. Redistribution, rebranding, or claiming this platform (or any variant or
          copy) as your own is prohibited.
        </p>
        <button class="btn btn-sm license-btn" onclick={() => (showLicense = true)}
          >View Full License</button
        >
      </div>
    </section>
  </div>
</div>

<style>
  .role-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-lg);
    margin-bottom: var(--space-md);
  }
  .role-now {
    font-size: var(--font-size-lg);
    font-weight: 600;
    color: var(--text-primary);
  }
  .key-val {
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    color: var(--text-primary);
    background: var(--bg-input);
    border: 1px solid var(--border-primary);
    border-radius: var(--radius-sm);
    padding: var(--space-sm) var(--space-md);
    word-break: break-all;
    margin: var(--space-sm) 0;
  }
  .key-actions {
    display: flex;
    gap: var(--space-sm);
  }
  .field {
    margin-bottom: var(--space-xl);
  }
  .field:last-child {
    margin-bottom: 0;
  }
  .field-desc code {
    font-family: var(--font-mono);
    font-size: 0.92em;
    color: var(--text-secondary);
    background: var(--bg-input);
    padding: 1px 5px;
    border-radius: var(--radius-sm);
  }
  .about-line {
    font-size: var(--font-size-sm);
    color: var(--text-secondary);
    line-height: 1.6;
    margin: 0 0 var(--space-sm);
  }
  .about-line.dim {
    font-size: var(--font-size-xs);
    color: var(--text-muted);
    margin-bottom: var(--space-md);
  }
  .license-btn {
    align-self: flex-start;
  }
  .update-note {
    font-size: var(--font-size-sm);
    color: var(--text-secondary);
    line-height: 1.6;
    margin: var(--space-md) 0 0;
  }
  .update-note.dim {
    color: var(--text-muted);
  }
  .update-actions {
    display: flex;
    gap: var(--space-sm);
    margin-top: var(--space-md);
    align-items: center;
  }
  .unsupported-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-sm);
    margin-top: var(--space-md);
  }
  .unsupported-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-md);
    padding: var(--space-md);
    border: 1px solid var(--color-yellow);
    border-radius: var(--radius-md, 8px);
  }
  .unsupported-name {
    color: var(--text-primary);
    font-weight: 600;
  }
  .unsupported-reason {
    color: var(--text-secondary);
    font-size: var(--font-size-sm);
    margin-top: 2px;
  }
</style>
