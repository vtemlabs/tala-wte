<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government use
  require a license from VTEM Labs. See the LICENSE file.
-->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { pb, system } from '$lib/api';
  import type { WirelessInterface } from '$lib/types';
  import GuideModal from '$lib/components/GuideModal.svelte';
  import { GUIDES } from '$lib/guides';

  let guideOpen = $state(false);

  type ClientStatus = {
    connected: boolean;
    ssid: string;
    interface: string;
    ip: string;
    gateway: string;
    portal_state: string;
    generating: boolean;
    requests: number;
    bytes_rx: number;
    errors: number;
    last_event?: string;
  };

  let status = $state<ClientStatus | null>(null);
  let interfaces = $state<WirelessInterface[]>([]);
  let unsupported = $state<{ usb_id: string; name: string; reason: string }[]>([]);
  let loading = $state(true);
  let poll: ReturnType<typeof setInterval> | null = null;

  const connected = $derived(status?.connected ?? false);
  const generating = $derived(status?.generating ?? false);

  function authHeaders(): Record<string, string> {
    return pb.authStore.token ? { Authorization: pb.authStore.token } : {};
  }

  function fmtBytes(n: number): string {
    if (!n) return '0 B';
    const u = ['B', 'KB', 'MB', 'GB'];
    let v = n;
    let i = 0;
    while (v >= 1024 && i < u.length - 1) {
      v /= 1024;
      i++;
    }
    return `${v.toFixed(i ? 1 : 0)} ${u[i]}`;
  }

  async function refreshStatus() {
    try {
      status = await fetch('/api/wte/client/status', { headers: authHeaders() }).then((r) => r.json());
    } catch {
      /* ignore transient poll errors */
    }
  }

  onMount(async () => {
    try {
      const sys = await system.interfaces();
      interfaces = sys.interfaces ?? [];
      unsupported = sys.unsupported ?? [];
    } catch {
      /* ignore */
    }
    await refreshStatus();
    loading = false;
    poll = setInterval(refreshStatus, 3000);
  });

  onDestroy(() => {
    if (poll) clearInterval(poll);
  });
</script>

<svelte:head><title>Dashboard - Tala WTE Client</title></svelte:head>

<div class="page-header">
  <div>
    <h1 class="page-title">Dashboard</h1>
    <p class="page-subtitle">Traffic generation agent status</p>
  </div>
  <div class="header-actions">
    <button class="btn" onclick={() => (guideOpen = true)}>Guide</button>
    <a href="/client/traffic" class="btn btn-primary">Open traffic console</a>
  </div>
</div>

<GuideModal bind:open={guideOpen} title={GUIDES.client.title} doc={GUIDES.client.doc} />

{#if !loading && unsupported.length > 0}
  <div class="error-toast hw-warn">
    <span>
      Wireless adapter(s) detected without driver support: {unsupported.map((a) => a.name).join(', ')}.
      Install the driver before {unsupported.length > 1 ? 'they' : 'it'} can connect.
    </span>
  </div>
{/if}

<div class="panel section">
  <div class="stat-strip">
    <div class="stat-cell">
      <span class="k">Connection</span>
      <span class="v" style={connected ? 'color:var(--color-green)' : 'color:var(--text-dim)'}>
        {#if connected}<span class="status-dot active"></span>{/if}{connected ? 'Online' : 'Offline'}
      </span>
    </div>
    <div class="stat-cell">
      <span class="k">Network</span>
      <span class="v">{status?.ssid || '-'}</span>
    </div>
    <div class="stat-cell">
      <span class="k">IP Address</span>
      <span class="v mono">{status?.ip || '-'}</span>
    </div>
    <div class="stat-cell">
      <span class="k">Wireless Adapters</span>
      <span class="v" style={interfaces.length ? 'color:var(--color-cyan)' : 'color:var(--color-yellow)'}>
        {interfaces.length}
      </span>
    </div>
  </div>
</div>

<div class="split-main">
  <div class="stack">
    <div class="panel">
      <div class="panel-head">
        <span class="panel-title">Connection</span>
        <span class="status-dot" class:active={connected} class:inactive={!connected}></span>
      </div>
      <div class="kv-list">
        <div class="kv-row"><span class="kv-k">Status</span><span class="kv-v">{connected ? 'Connected' : 'Offline'}</span></div>
        <div class="kv-row"><span class="kv-k">SSID</span><span class="kv-v">{status?.ssid || '-'}</span></div>
        <div class="kv-row"><span class="kv-k">Interface</span><span class="kv-v mono">{status?.interface || '-'}</span></div>
        <div class="kv-row"><span class="kv-k">IP address</span><span class="kv-v mono">{status?.ip || '-'}</span></div>
        <div class="kv-row"><span class="kv-k">Gateway</span><span class="kv-v mono">{status?.gateway || '-'}</span></div>
        <div class="kv-row"><span class="kv-k">Captive portal</span><span class="kv-v">{status?.portal_state || 'none'}</span></div>
      </div>
    </div>

    <div class="panel">
      <div class="panel-head">
        <span class="panel-title">Traffic Generation</span>
        <a href="/client/traffic" class="action-btn">Console</a>
      </div>
      <div class="stat-strip inner">
        <div class="stat-cell">
          <span class="k">Generating</span>
          <span class="v" style={generating ? 'color:var(--color-green)' : ''}>
            {#if generating}<span class="status-dot active"></span>{/if}{generating ? 'Active' : 'Idle'}
          </span>
        </div>
        <div class="stat-cell"><span class="k">Requests</span><span class="v">{status?.requests ?? 0}</span></div>
        <div class="stat-cell"><span class="k">Received</span><span class="v">{fmtBytes(status?.bytes_rx ?? 0)}</span></div>
        <div class="stat-cell">
          <span class="k">Errors</span>
          <span class="v" style={(status?.errors ?? 0) > 0 ? 'color:var(--color-orange)' : ''}>{status?.errors ?? 0}</span>
        </div>
      </div>
    </div>
  </div>

  <div class="panel">
    <div class="panel-head">
      <span class="panel-title">Wireless Interfaces</span>
      <span class="count-pill">{interfaces.length}</span>
    </div>
    {#if loading}
      <div class="empty-state" style="padding:var(--space-xl)"><p>Loading...</p></div>
    {:else if interfaces.length === 0}
      <div class="empty-state" style="padding:var(--space-xl)">
        <p>No wireless adapter detected. Plug in a USB adapter to join a network.</p>
      </div>
    {:else}
      <div class="rail-list">
        {#each interfaces as iface}
          <div class="rail-row">
            <span class="status-dot active"></span>
            <div class="rail-body">
              <div class="mono rail-name">{iface.interface}</div>
              <div class="dim rail-sub">
                {#if iface.manufacturer || iface.device_model}{iface.manufacturer}
                  {iface.device_model}{:else if iface.driver}{iface.driver}{/if}
              </div>
            </div>
            {#if iface.interface === status?.interface && connected}
              <span class="count-pill">connected</span>
            {:else if iface.chipset}
              <span class="count-pill">{iface.chipset}</span>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>

<style>
  .hw-warn {
    border-color: var(--color-yellow);
    background: rgba(234, 179, 8, 0.08);
    color: var(--color-yellow);
  }
  .stat-strip.inner {
    padding: 0;
    background: transparent;
    border: none;
  }
  .kv-list {
    display: flex;
    flex-direction: column;
  }
  .kv-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--space-md);
    padding: 10px var(--space-lg);
    border-bottom: 1px solid var(--border-primary);
  }
  .kv-row:last-child {
    border-bottom: none;
  }
  .kv-k {
    font-size: var(--font-size-xs);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--text-dim);
  }
  .kv-v {
    color: var(--text-primary);
    font-size: var(--font-size-sm);
  }
  .rail-list {
    display: flex;
    flex-direction: column;
    padding: var(--space-sm) var(--space-xl);
  }
  .rail-row {
    display: flex;
    align-items: center;
    gap: var(--space-md);
    padding: var(--space-md) 0;
    border-bottom: 1px solid var(--border-subtle);
  }
  .rail-row:last-child {
    border-bottom: none;
  }
  .rail-body {
    min-width: 0;
    flex: 1;
  }
  .rail-name {
    font-size: var(--font-size-sm);
    font-weight: 600;
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .rail-sub {
    font-size: var(--font-size-xs);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
