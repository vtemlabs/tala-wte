<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government
  use require a license from VTEM Labs. The Software may not be copied or
  redistributed. See the LICENSE file.
-->
<script lang="ts">
  import { page } from '$app/state';
  import { onMount, onDestroy } from 'svelte';
  import { networks, pb } from '$lib/api';
  import { toast } from '$lib/stores/toast';
  import ProtocolGuide from '$lib/ProtocolGuide.svelte';
  import EnterprisePreflight from '$lib/EnterprisePreflight.svelte';
  import RadioSwapModal from '$lib/components/RadioSwapModal.svelte';
  import LogWindow from '$lib/components/LogWindow.svelte';
  import type { WirelessClient } from '$lib/types';
  import type { AdapterSwap } from '$lib/api';
  import { protocolBadge } from '$lib/protocol';

  let logPopped = $state(false);

  const enterpriseProtocols = ['wpa2_enterprise', 'wpa3_enterprise'];
  let preflightOpen = $state(false);

  // Radio-management swap prompt (saved adapter gone, propose an available one).
  let swapOpen = $state(false);
  let swapData = $state<AdapterSwap | null>(null);
  let swapBusy = $state(false);

  const id = $derived(page.params.id ?? '');
  let net = $state<Record<string, any> | null>(null);
  let clients = $state<WirelessClient[]>([]);
  let loading = $state(true);
  let error = $state('');
  let guideCollapsed = $state(false);
  let toggling = $state(false);
  let pollInterval: ReturnType<typeof setInterval> | null = null;
  let pollFailures = 0;
  let logs = $state<string[]>([]);
  let logEl: HTMLElement | undefined = $state();

  async function fetchLogs() {
    try {
      const r = await fetch(`/api/wte/networks/${id}/logs`, {
        headers: pb.authStore.token ? { Authorization: pb.authStore.token } : {}
      });
      if (r.ok) {
        const atBottom = !logEl || logEl.scrollTop + logEl.clientHeight >= logEl.scrollHeight - 40;
        logs = (await r.json()).lines ?? [];
        if (atBottom)
          requestAnimationFrame(() => {
            if (logEl) logEl.scrollTop = logEl.scrollHeight;
          });
      }
    } catch {
      /* keep last logs */
    }
  }

  // Self-scheduling poll so the delay can back off after failures (a fixed setInterval period cannot).
  async function pollOnce() {
    try {
      const [statusRes, clientRes] = await Promise.all([
        fetch(`/api/wte/networks/${id}/status`, {
          headers: pb.authStore.token ? { Authorization: pb.authStore.token } : {}
        }).then((r) => {
          if (!r.ok) throw new Error(`HTTP ${r.status}`);
          return r.json();
        }),
        networks.clients(id)
      ]);
      if (net) {
        net = { ...net, status: statusRes.status };
      }
      clients = clientRes.clients ?? [];
      fetchLogs();
      if (pollFailures > 0) {
        toast.info('Connection restored');
      }
      pollFailures = 0;
    } catch {
      pollFailures++;
      if (pollFailures === 3) {
        toast.err('Lost connection to server - retrying in background');
      }
      // Keep polling - back off to 15s after repeated failures.
    }
    // Reschedule only if polling was not stopped during the await.
    if (pollInterval !== null) {
      pollInterval = setTimeout(pollOnce, pollFailures >= 3 ? 15000 : 5000);
    }
  }

  function startPolling() {
    stopPolling();
    pollFailures = 0;
    fetchLogs();
    pollInterval = setTimeout(pollOnce, 5000);
  }

  function stopPolling() {
    if (pollInterval) {
      clearTimeout(pollInterval);
      pollInterval = null;
    }
  }

  onMount(async () => {
    try {
      net = await networks.get(id);
      const res = await networks.clients(id);
      clients = res.clients ?? [];
      fetchLogs();
      if (net?.status === 'running') {
        startPolling();
      }
    } catch (e: any) {
      error = e?.message ?? 'Failed to load network';
    }
    loading = false;
  });

  onDestroy(() => stopPolling());

  async function exportClientConfig() {
    try {
      const r = await fetch(`/api/wte/networks/${id}/client-config`, {
        headers: pb.authStore.token ? { Authorization: pb.authStore.token } : {}
      });
      if (!r.ok) throw new Error('export failed');
      const blob = await r.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `tala-client-${(net?.ssid || 'network').replace(/[^A-Za-z0-9._-]+/g, '_')}.json`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    } catch {
      error = 'Failed to export client config';
    }
  }

  async function toggleNetwork() {
    if (!net) return;

    // Stop is unconditional; start for enterprise protocols routes through the modal.
    if (net.status === 'running') {
      toggling = true;
      try {
        await networks.stop(id);
        net = { ...net, status: 'stopped' };
        stopPolling();
      } catch (e: any) {
        error = e?.message ?? 'Failed to stop network';
      }
      toggling = false;
      return;
    }

    if (enterpriseProtocols.includes(net.protocol)) {
      preflightOpen = true;
      return;
    }

    toggling = true;
    try {
      await networks.start(id);
      net = { ...net, status: 'running' };
      const res = await networks.clients(id);
      clients = res.clients ?? [];
      startPolling();
    } catch (e: any) {
      if (e?.adapterSwap) {
        swapData = e.adapterSwap;
        swapOpen = true;
      } else {
        error = e?.message ?? 'Failed to start network';
      }
    }
    toggling = false;
  }

  // Operator confirmed the proposed adapter (and any band change) from the swap modal.
  async function confirmSwap(): Promise<void> {
    if (!swapData || !net) return;
    swapBusy = true;
    try {
      await networks.start(id, {
        interface: swapData.proposed.interface,
        band: swapData.band_ok ? '' : swapData.suggested_band
      });
      net = {
        ...net,
        status: 'running',
        interface: swapData.proposed.interface,
        band: swapData.band_ok ? net.band : swapData.suggested_band
      };
      const res = await networks.clients(id);
      clients = res.clients ?? [];
      startPolling();
      swapOpen = false;
    } catch (e: any) {
      swapOpen = false;
      error = e?.message ?? 'Failed to start network';
    }
    swapBusy = false;
  }

  async function startFromPreflight(autoProvision: boolean): Promise<void> {
    if (!net) return;
    await networks.start(id, { autoProvision });
    net = { ...net, status: 'running' };
    const res = await networks.clients(id);
    clients = res.clients ?? [];
    startPolling();
  }
</script>

<svelte:head><title>{net?.ssid ?? 'Network'} - Tala WTE</title></svelte:head>

<div class="page-header">
  <div>
    <a href="/networks" class="back-link">← Networks</a>
    {#if net}
      <h1 class="page-title">{net.ssid}</h1>
    {/if}
  </div>
  {#if net}
    <div class="header-actions">
      <button
        class="btn btn-secondary"
        onclick={exportClientConfig}
        title="Download a config to import into a Tala WTE client"
      >
        Export client config
      </button>
      <button
        class="btn"
        class:btn-success={net.status !== 'running'}
        class:btn-danger={net.status === 'running'}
        onclick={toggleNetwork}
        disabled={toggling}
      >
        {toggling ? '…' : net.status === 'running' ? 'Stop Network' : 'Start Network'}
      </button>
    </div>
  {/if}
</div>

{#if error}
  <div class="error-toast"><span>{error}</span></div>
{/if}

{#if loading}
  <div class="empty-state"><p>Loading…</p></div>
{:else if net}
  <div class="detail-layout">
    <div class="detail-main stack">
      <div class="status-bar">
        <div class="sb-item">
          <span class="sb-label">Status</span>
          <span class="sb-value"
            ><span
              class="status-dot"
              class:active={net.status === 'running'}
              class:inactive={net.status === 'stopped'}
              class:error={net.status === 'error'}
            ></span><span style="text-transform:capitalize">{net.status}</span></span
          >
        </div>
        <div class="sb-item">
          <span class="sb-label">Protocol</span>
          <span class="sb-value"
            ><span class="badge {protocolBadge(net.protocol)}"
              >{net.protocol.replace('_', '-').toUpperCase()}</span
            ></span
          >
        </div>
        <div class="sb-item">
          <span class="sb-label">Band</span>
          <span class="sb-value">{net.band ?? '2.4'} GHz</span>
        </div>
        <div class="sb-item">
          <span class="sb-label">Channel</span>
          <span class="sb-value">{net.channel ?? 6}</span>
        </div>
        <div class="sb-item">
          <span class="sb-label">Clients</span>
          <span class="sb-value">{clients.length}</span>
        </div>
      </div>

      <div class="panel">
        <div class="panel-head"><h2 class="panel-title">Configuration</h2></div>
        <div class="panel-body">
          <div class="meta-grid">
            <div class="meta-row">
              <div class="meta-key">SSID</div>
              <div class="meta-val">{net.ssid}</div>
            </div>
            <div class="meta-row">
              <div class="meta-key">Protocol</div>
              <div class="meta-val">{net.protocol}</div>
            </div>
            <div class="meta-row">
              <div class="meta-key">Band</div>
              <div class="meta-val">{net.band ?? '2.4'} GHz</div>
            </div>
            <div class="meta-row">
              <div class="meta-key">Channel</div>
              <div class="meta-val">{net.channel ?? 6}</div>
            </div>
            <div class="meta-row">
              <div class="meta-key">Interface</div>
              <div class="meta-val">{net.interface || '-'}</div>
            </div>
            <div class="meta-row">
              <div class="meta-key">Isolation</div>
              <div class="meta-val">{net.client_isolation ? 'Enabled' : 'Disabled'}</div>
            </div>
            <div class="meta-row">
              <div class="meta-key">NAT</div>
              <div class="meta-val">{net.internet_passthrough ? 'Enabled' : 'Disabled'}</div>
            </div>
          </div>
        </div>
      </div>

      {#if clients.length > 0}
        <div class="panel">
          <div class="panel-head">
            <h2 class="panel-title">Connected Clients</h2>
            <span class="count-pill">{clients.length}</span>
          </div>
          <div class="table-wrap">
            <table class="table">
              <thead><tr><th>MAC</th><th>IP</th><th>Signal</th></tr></thead>
              <tbody>
                {#each clients as c}
                  <tr>
                    <td data-label="MAC" class="mono">{c.mac}</td>
                    <td data-label="IP" class="mono">{c.ip ?? '-'}</td>
                    <td data-label="Signal" class="mono">{c.signal} dBm</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        </div>
      {/if}

      <div class="panel">
        <div class="panel-head">
          <h2 class="panel-title">Live Log</h2>
          <div class="log-head-right">
            <span class="log-status"
              ><span
                class="status-dot"
                class:active={net.status === 'running'}
                class:inactive={net.status !== 'running'}
              ></span>{net.status === 'running' ? 'streaming' : 'idle'}</span
            >
            <button
              class="action-btn"
              onclick={() => (logPopped = true)}
              title="Pop out the log into a resizable window">Pop out</button
            >
          </div>
        </div>
        <div class="panel-body logbody">
          <div class="logbox" bind:this={logEl}>
            {#if logs.length === 0}
              <div class="logempty">No activity yet - start the network and connect a client.</div>
            {:else}
              {#each logs as line}<div class="logline">{line}</div>{/each}
            {/if}
          </div>
        </div>
      </div>
    </div>

    <ProtocolGuide
      protocol={net.protocol}
      collapsed={guideCollapsed}
      onToggle={() => (guideCollapsed = !guideCollapsed)}
    />
  </div>
{/if}

{#if net}
  <EnterprisePreflight
    open={preflightOpen}
    ssid={net.ssid}
    onClose={() => (preflightOpen = false)}
    onStart={startFromPreflight}
  />
  <RadioSwapModal
    bind:open={swapOpen}
    swap={swapData}
    busy={swapBusy}
    onConfirm={confirmSwap}
    onCancel={() => (swapOpen = false)}
  />
  <LogWindow
    bind:open={logPopped}
    title={`Live Log - ${net.ssid}`}
    streaming={net.status === 'running'}
    lines={logs}
  />
{/if}

<style>
  .page-header {
    align-items: center;
    margin-bottom: var(--space-xl);
  }
  .back-link {
    font-size: var(--font-size-xs);
    color: var(--text-dim);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  .back-link:hover {
    color: var(--text-secondary);
  }
  .page-header .page-title {
    margin-top: 2px;
    font-size: var(--font-size-xl);
  }

  .detail-main {
    flex: 1;
    min-width: 0;
  }

  /* Compact status strip - framed, divided cells (kept intact). */
  .status-bar {
    display: flex;
    flex-wrap: wrap;
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.025), transparent 120px), var(--bg-card);
    border: 1px solid var(--border-primary);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-md);
    overflow: hidden;
  }
  .sb-item {
    flex: 1;
    min-width: 110px;
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: var(--space-md) var(--space-xl);
    border-right: 1px solid var(--border-primary);
  }
  .sb-item:last-child {
    border-right: none;
  }
  .sb-label {
    font-size: var(--font-size-2xs);
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-muted);
  }
  .sb-value {
    display: flex;
    align-items: center;
    gap: 7px;
    font-size: var(--font-size-lg);
    font-weight: 600;
    color: var(--text-primary);
    font-variant-numeric: tabular-nums;
  }
  .sb-value .status-dot {
    margin: 0;
  }
  .sb-value .badge {
    font-size: var(--font-size-xs);
  }
  @media (max-width: 760px) {
    .sb-item {
      flex: 1 1 33%;
      border-bottom: 1px solid var(--border-primary);
    }
  }

  .log-head-right {
    display: flex;
    align-items: center;
    gap: var(--space-md);
  }
  .log-status {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: var(--font-size-2xs);
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-muted);
  }
  .log-status .status-dot {
    margin: 0;
  }

  /* Live log body sits flush as a dark terminal inside the panel. */
  .logbody {
    padding: 0;
  }
  .logbox {
    height: 460px;
    overflow-y: auto;
    background: var(--bg-primary);
    padding: var(--space-md) var(--space-lg);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    line-height: 1.55;
    color: #c9d1d9;
    white-space: pre-wrap;
    word-break: break-word;
  }
  .logline {
    color: #c9d1d9;
  }
  .logline:hover {
    color: var(--text-primary);
  }
  .logempty {
    color: var(--text-dim);
    font-style: italic;
  }
</style>
