<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, paid training, paid CTF,
  or any for-profit use requires a license from VTEM Labs. See the LICENSE file.
-->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { pb, networks, system } from '$lib/api';
  import { toast } from '$lib/stores/toast';
  import type { WirelessInterface } from '$lib/types';
  import { protocolBadge } from '$lib/protocol';

  let networkList = $state<Record<string, any>[]>([]);
  let interfaces = $state<WirelessInterface[]>([]);
  let inUse = $state<Record<string, string>>({});
  let unsupported = $state<{ usb_id: string; name: string; reason: string }[]>([]);
  let loading = $state(true);
  let unsubscribe: (() => void) | null = null;

  const activeNets = $derived(networkList.filter((n) => n.status === 'running'));
  const totalClients = $derived(networkList.reduce((s, n) => s + (n.client_count ?? 0), 0));
  // Adapters in use by a running network have their PHY moved into that network's
  // namespace, so the host-level scan does not see them; count them too.
  const inUseList = $derived(Object.entries(inUse).map(([iface, ssid]) => ({ iface, ssid })));
  const totalAdapters = $derived(interfaces.length + inUseList.length);
  const hasRealHardware = $derived(totalAdapters > 0);

  const PROTO_COLOR: Record<string, string> = {
    open: 'var(--text-muted)',
    wep: 'var(--color-orange)',
    wpa: 'var(--color-orange)',
    wpa2: 'var(--color-yellow)',
    wps: 'var(--color-red)',
    wpa3: 'var(--color-green)',
    wpa3_transition: 'var(--color-green)',
    wpa2_enterprise: 'var(--color-purple)',
    wpa3_enterprise: 'var(--color-purple)'
  };
  const protoDist = $derived.by(() => {
    const m: Record<string, number> = {};
    for (const n of networkList) m[n.protocol] = (m[n.protocol] ?? 0) + 1;
    return Object.entries(m).sort((a, b) => b[1] - a[1]);
  });
  const protoLabel = (p: string) => p.replace('_', '-').toUpperCase();

  onMount(async () => {
    try {
      const [nets, sys] = await Promise.all([networks.list(), system.interfaces()]);
      networkList = nets;
      interfaces = sys.interfaces ?? [];
      inUse = sys.in_use ?? {};
      unsupported = sys.unsupported ?? [];
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to load dashboard data');
    }
    loading = false;

    unsubscribe = await pb.collection('networks').subscribe('*', (e) => {
      if (e.action === 'update') {
        networkList = networkList.map((n) => (n.id === e.record.id ? e.record : n));
      } else if (e.action === 'create') {
        networkList = [e.record, ...networkList];
      } else if (e.action === 'delete') {
        networkList = networkList.filter((n) => n.id !== e.record.id);
      }
    });
  });

  onDestroy(() => unsubscribe?.());
</script>

<svelte:head><title>Dashboard - Tala WTE</title></svelte:head>

<div class="page-header">
  <div>
    <h1 class="page-title">Dashboard</h1>
    <p class="page-subtitle">Wireless Training Environment status overview</p>
  </div>
  <a href="/networks/new" class="btn btn-primary">+ New Network</a>
</div>

{#if !loading && !hasRealHardware}
  <div class="error-toast hw-warn">
    <span
      >No wireless hardware detected. Networks will not broadcast over the air until a USB wireless
      adapter is plugged in.</span
    >
  </div>
{/if}

{#if !loading && unsupported.length > 0}
  <div class="error-toast hw-warn">
    <span>
      Wireless adapter(s) detected without driver support: {unsupported.map((a) => a.name).join(', ')}.
      Find and install the driver/firmware for {unsupported.length > 1 ? 'these adapters' : 'this adapter'}
      before {unsupported.length > 1 ? 'they' : 'it'} can be used as a radio.
    </span>
  </div>
{/if}

<div class="panel section">
  <div class="stat-strip">
    <div class="stat-cell">
      <span class="k">Total Networks</span>
      <span class="v">{networkList.length}</span>
    </div>
    <div class="stat-cell">
      <span class="k">Active Networks</span>
      <span class="v" style={activeNets.length ? 'color:var(--color-green)' : ''}>
        {#if activeNets.length}<span class="status-dot active"></span>{/if}{activeNets.length}
      </span>
    </div>
    <div class="stat-cell">
      <span class="k">Connected Clients</span>
      <span class="v">{totalClients}</span>
    </div>
    <div class="stat-cell">
      <span class="k">Wireless Interfaces</span>
      <span
        class="v"
        style={hasRealHardware ? 'color:var(--color-cyan)' : 'color:var(--color-yellow)'}
      >
        {totalAdapters}
      </span>
    </div>
  </div>
</div>

{#if protoDist.length > 0}
  <div class="panel section">
    <div class="panel-head">
      <span class="panel-title">Protocol Distribution</span>
      <span class="count-pill">{protoDist.length} protocol{protoDist.length === 1 ? '' : 's'}</span>
    </div>
    <div class="panel-body">
      <div class="dist-bar">
        {#each protoDist as [proto, n]}
          <div
            class="dist-seg"
            style="flex:{n};background:{PROTO_COLOR[proto] ?? 'var(--text-dim)'}"
            title="{protoLabel(proto)}: {n}"
          ></div>
        {/each}
      </div>
      <div class="dist-legend">
        {#each protoDist as [proto, n]}
          <span class="dist-key"
            ><span class="dist-dot" style="background:{PROTO_COLOR[proto] ?? 'var(--text-dim)'}"
            ></span>{protoLabel(proto)} <b>{n}</b></span
          >
        {/each}
      </div>
    </div>
  </div>
{/if}

<div class="split-main">
  <div class="panel">
    <div class="panel-head">
      <span class="panel-title">Networks</span>
      <a href="/networks" class="action-btn">View all</a>
    </div>
    {#if loading}
      <div class="empty-state"><p>Loading…</p></div>
    {:else if networkList.length === 0}
      <div class="empty-state">
        <p>No networks configured yet.</p>
        <a href="/networks/new" class="btn btn-primary" style="margin-top:var(--space-lg)"
          >Create First Network</a
        >
      </div>
    {:else}
      <div class="table-wrap">
        <table class="table">
          <thead><tr><th>SSID</th><th>Protocol</th><th>Band</th><th>Status</th><th></th></tr></thead
          >
          <tbody>
            {#each networkList as net}
              <tr>
                <td><a href="/networks/{net.id}" class="mono ssid-link">{net.ssid}</a></td>
                <td
                  ><span class="badge {protocolBadge(net.protocol)}"
                    >{protoLabel(net.protocol)}</span
                  ></td
                >
                <td class="dim">{net.band ?? '2.4'} GHz</td>
                <td
                  ><span class="stat-status"
                    ><span
                      class="status-dot"
                      class:active={net.status === 'running'}
                      class:inactive={net.status === 'stopped'}
                      class:error={net.status === 'error'}
                    ></span>{net.status}</span
                  ></td
                >
                <td style="text-align:right"
                  ><a href="/networks/{net.id}" class="action-btn">View</a></td
                >
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>

  <div class="stack">
    <div class="panel">
      <div class="panel-head">
        <span class="panel-title">Wireless Interfaces</span>
        <span class="count-pill">{totalAdapters}</span>
      </div>
      {#if totalAdapters === 0}
        <div class="empty-state" style="padding:var(--space-xl)">
          <p>No interfaces detected. Plug in a USB adapter.</p>
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
              {#if iface.chipset}<span class="count-pill">{iface.chipset}</span>{/if}
            </div>
          {/each}
          {#each inUseList as a}
            <div class="rail-row">
              <span class="status-dot active"></span>
              <div class="rail-body">
                <div class="mono rail-name">{a.iface}</div>
                <div class="dim rail-sub">in use by {a.ssid}</div>
              </div>
              <span class="count-pill">in use</span>
            </div>
          {/each}
        </div>
      {/if}
    </div>

    <div class="panel">
      <div class="panel-head">
        <span class="panel-title">Services</span>
        <span class="status-dot active"></span>
      </div>
      <div class="rail-list">
        <div class="rail-row svc-row">
          <span class="status-dot active"></span><span class="svc-name">FreeRADIUS</span><span
            class="svc-state">online</span
          >
        </div>
        <div class="rail-row svc-row">
          <span class="status-dot active"></span><span class="svc-name">OpenLDAP</span><span
            class="svc-state">online</span
          >
        </div>
        <div class="rail-row svc-row">
          <span class="status-dot active"></span><span class="svc-name">Portal server</span><span
            class="svc-state">online</span
          >
        </div>
        <div class="rail-row svc-row">
          <span class="status-dot active"></span><span class="svc-name">PocketBase</span><span
            class="svc-state">online</span
          >
        </div>
      </div>
    </div>

    <div class="panel">
      <div class="panel-head">
        <span class="panel-title">Quick Actions</span>
      </div>
      <div class="quick-grid">
        <a href="/networks/new" class="quick-btn">New Network</a>
        <a href="/portals" class="quick-btn">Captive Portals</a>
        <a href="/captures" class="quick-btn">Start Capture</a>
        <a href="/ldap" class="quick-btn">LDAP Users</a>
      </div>
    </div>
  </div>
</div>

<style>
  .hw-warn {
    border-color: var(--color-yellow);
    background: rgba(234, 179, 8, 0.08);
    color: var(--color-yellow);
  }

  .stat-cell .v {
    line-height: 1.1;
  }
  .stat-cell .v .status-dot {
    margin: 0;
    flex-shrink: 0;
  }
  .cell-sub {
    font-size: var(--font-size-xs);
    font-weight: 500;
    color: var(--text-dim);
    margin-left: 2px;
    letter-spacing: 0;
    text-transform: none;
    align-self: center;
  }

  .dist-bar {
    display: flex;
    height: 12px;
    border-radius: 999px;
    overflow: hidden;
    gap: 2px;
    background: var(--bg-input);
  }
  .dist-seg {
    min-width: 6px;
    border-radius: 2px;
    transition: flex var(--transition-base);
  }
  .dist-legend {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-md) var(--space-lg);
    margin-top: var(--space-md);
  }
  .dist-key {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: var(--font-size-xs);
    color: var(--text-muted);
    font-weight: 500;
  }
  .dist-key b {
    color: var(--text-primary);
    font-family: var(--font-mono);
  }
  .dist-dot {
    width: 8px;
    height: 8px;
    border-radius: 2px;
    flex-shrink: 0;
  }

  .ssid-link {
    color: var(--text-primary);
    font-weight: 600;
  }
  .stat-status {
    display: inline-flex;
    align-items: center;
    text-transform: capitalize;
  }

  .rail-list {
    display: flex;
    flex-direction: column;
    padding: var(--space-sm) var(--space-xl);
  }
  .rail-row {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
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
  }
  .rail-sub {
    font-size: var(--font-size-xs);
  }

  .svc-row .svc-name {
    flex: 1;
    font-size: var(--font-size-sm);
    font-weight: 500;
    color: var(--text-primary);
    min-width: 0;
  }
  .svc-row .svc-state {
    font-size: var(--font-size-xs);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--color-green);
  }

  .quick-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-sm);
    padding: var(--space-xl);
  }
  .quick-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 12px;
    font-size: var(--font-size-sm);
    font-weight: 500;
    color: var(--text-secondary);
    background: var(--bg-tertiary);
    border: 1px solid var(--border-primary);
    border-radius: var(--radius-md);
    transition: all var(--transition-fast);
    text-align: center;
  }
  .quick-btn:hover {
    color: var(--text-primary);
    border-color: var(--accent);
    background: var(--accent-soft);
  }
</style>
