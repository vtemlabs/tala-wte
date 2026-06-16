<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, paid training, paid CTF,
  or any for-profit use requires a license from VTEM Labs. See the LICENSE file.
-->
<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { networks } from '$lib/api';
  import { pb } from '$lib/api';
  import { toast } from '$lib/stores/toast';
  import EnterprisePreflight from '$lib/EnterprisePreflight.svelte';
  import { protocolBadge } from '$lib/protocol';

  let list = $state<Record<string, any>[]>([]);
  let filter = $state('');
  let loading = $state(true);
  let error = $state('');
  let unsubscribe: (() => void) | null = null;

  let preflightOpen = $state(false);
  let preflightTarget = $state<Record<string, any> | null>(null);

  const enterpriseProtocols = ['wpa2_enterprise', 'wpa3_enterprise'];

  const filtered = $derived(filter
    ? list.filter(n => n.ssid.toLowerCase().includes(filter.toLowerCase()) || n.protocol.includes(filter))
    : list
  );

  onMount(async () => {
    try {
      list = await networks.list();
    } catch (e: any) {
      error = e?.message ?? 'Failed to load networks';
    }
    loading = false;

    try {
      unsubscribe = await pb.collection('networks').subscribe('*', async (e) => {
        if (e.action === 'create') {
          list = await networks.list();
        } else if (e.action === 'update') {
          const idx = list.findIndex(n => n.id === e.record.id);
          if (idx !== -1) {
            list[idx] = e.record;
            list = [...list];
          }
        } else if (e.action === 'delete') {
          list = list.filter(n => n.id !== e.record.id);
        }
      });
    } catch {
      // Realtime not critical - page still works with manual refresh
    }
  });

  onDestroy(() => unsubscribe?.());

  async function toggleNetwork(net: Record<string, any>) {
    if (net.status === 'running') {
      try {
        await networks.stop(net.id);
        toast.info(`Stopped network ${net.ssid}`);
        list = await networks.list();
      } catch (e: any) {
        toast.err(e.message || 'An unknown error occurred');
        list = await networks.list();
      }
      return;
    }

    // Enterprise networks route through the preflight modal for the missing-deps and auto-provision flow.
    if (enterpriseProtocols.includes(net.protocol)) {
      preflightTarget = net;
      preflightOpen = true;
      return;
    }

    try {
      await networks.start(net.id);
      toast.success(`Successfully started ${net.ssid}`);
      list = await networks.list();
    } catch (e: any) {
      toast.err(e.message || 'An unknown error occurred');
      list = await networks.list();
    }
  }

  async function startFromPreflight(autoProvision: boolean): Promise<void> {
    if (!preflightTarget) return;
    await networks.start(preflightTarget.id, { autoProvision });
    toast.success(`Successfully started ${preflightTarget.ssid}`);
    list = await networks.list();
  }

  function closePreflight() {
    preflightOpen = false;
    preflightTarget = null;
  }

  async function deleteNetwork(id: string) {
    if (!confirm('Delete this network?')) return;
    await networks.delete(id);
    list = list.filter(n => n.id !== id);
  }

</script>

<svelte:head><title>Networks - Tala WTE</title></svelte:head>

<div class="page-header">
  <div>
    <h1 class="page-title">Networks</h1>
    <p class="page-subtitle">Manage wireless access point configurations</p>
  </div>
  <div class="header-actions">
    <a href="/networks/guide" class="btn">Guide</a>
    <a href="/networks/new" class="btn btn-primary">+ New Network</a>
  </div>
</div>

<div class="panel">
  <div class="panel-head net-head">
    <input class="input filter-field" type="text" bind:value={filter} placeholder="Filter by SSID or protocol…" />
    <span class="count-pill">{filtered.length} / {list.length}</span>
  </div>

  {#if loading}
    <div class="empty-state"><p>Loading networks…</p></div>
  {:else if filtered.length === 0}
    <div class="empty-state">
      <div class="empty-icon">
        <svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12.55a11 11 0 0 1 14.08 0"></path><path d="M1.42 9a16 16 0 0 1 21.16 0"></path><path d="M8.53 16.11a6 6 0 0 1 6.95 0"></path><line x1="12" y1="20" x2="12.01" y2="20"></line></svg>
      </div>
      <p>{filter ? 'No networks match your filter.' : 'No networks configured yet.'}</p>
      {#if !filter}
        <a href="/networks/new" class="btn btn-primary" style="margin-top:var(--space-lg)">Create First Network</a>
      {/if}
    </div>
  {:else}
    <div class="table-wrap">
      <table class="table net-table">
        <thead>
          <tr>
            <th>SSID</th>
            <th>Protocol</th>
            <th>Band</th>
            <th>Channel</th>
            <th>Interface</th>
            <th>Status</th>
            <th class="actions-col">Actions</th>
          </tr>
        </thead>
        <tbody>
          {#each filtered as net}
            <tr>
              <td><a href="/networks/{net.id}" class="mono ssid-link">{net.ssid}</a></td>
              <td><span class="badge {protocolBadge(net.protocol)}">{net.protocol.replace('_', '-').toUpperCase()}</span></td>
              <td class="dim">{net.band ?? '2.4'} GHz</td>
              <td class="mono dim">{net.channel ?? 6}</td>
              <td class="mono dim">{net.interface || '-'}</td>
              <td>
                <span class="net-status">
                  <span class="status-dot" class:active={net.status === 'running'} class:inactive={net.status === 'stopped'} class:error={net.status === 'error'}></span>{net.status}
                </span>
              </td>
              <td class="actions-col">
                <div class="row-actions">
                  <button class="action-btn" class:btn-success={net.status !== 'running'} class:btn-danger={net.status === 'running'} onclick={() => toggleNetwork(net)}>
                    {net.status === 'running' ? 'Stop' : 'Start'}
                  </button>
                  <a href="/networks/{net.id}" class="action-btn">Details</a>
                  <button class="action-btn del-btn" onclick={() => deleteNetwork(net.id)}>Del</button>
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<EnterprisePreflight
  open={preflightOpen}
  ssid={preflightTarget?.ssid ?? ''}
  onClose={closePreflight}
  onStart={startFromPreflight}
/>

<style>
  .net-head { gap: var(--space-md); }
  .filter-field { max-width: 340px; }

  .empty-icon { margin-bottom: var(--space-lg); color: var(--text-dim); }

  .table-wrap { overflow-x: auto; }
  .ssid-link { color: var(--text-primary); font-weight: 600; }

  .net-table th.actions-col,
  .net-table td.actions-col { text-align: right; white-space: nowrap; }

  .net-status { display: inline-flex; align-items: center; text-transform: capitalize; white-space: nowrap; }

  .row-actions { display: flex; gap: 4px; justify-content: flex-end; }
  .del-btn { color: var(--color-red); }
  .del-btn:hover { color: #fff; background: var(--color-red); border-color: transparent; }
</style>
