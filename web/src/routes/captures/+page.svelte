<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government
  use require a license from VTEM Labs. The Software may not be copied or
  redistributed. See the LICENSE file.
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { captures, networks, system } from '$lib/api';
  import { toast } from '$lib/stores/toast';
  import type { WirelessInterface } from '$lib/types';
  import GuideModal from '$lib/components/GuideModal.svelte';
  import { GUIDES } from '$lib/guides';

  let guideOpen = $state(false);
  let captureList = $state<Record<string, any>[]>([]);
  let networkList = $state<Record<string, any>[]>([]);
  let interfaces = $state<WirelessInterface[]>([]);
  let loading = $state(true);

  let selectedNet = $state('');
  let layer = $state<'wireless' | 'network'>('network');
  let iface = $state('');
  let filter = $state('');
  let starting = $state(false);
  let error = $state('');

  const FILTER_PRESETS = [
    { label: 'HTTP', bpf: 'tcp port 80' },
    { label: 'TLS / HTTPS', bpf: 'tcp port 443' },
    { label: 'DNS', bpf: 'udp port 53' },
    { label: 'DHCP', bpf: 'udp port 67 or udp port 68' },
    { label: 'ARP', bpf: 'arp' },
    { label: 'ICMP', bpf: 'icmp' },
    { label: 'Clear', bpf: '' }
  ];

  onMount(async () => {
    try {
      [captureList, networkList, { interfaces }] = await Promise.all([
        captures.list(),
        networks.list(),
        system.interfaces()
      ]);
      if (interfaces.length) iface = interfaces[0].interface;
      if (networkList.length) selectedNet = networkList[0].id;
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to load capture data');
    } finally {
      loading = false;
    }
  });

  async function start() {
    starting = true;
    error = '';
    try {
      await captures.start(selectedNet, layer, iface, filter);
      captureList = await captures.list();
    } catch (e: any) {
      error = e?.message ?? 'Failed to start capture';
    }
    starting = false;
  }

  async function stop(id: string) {
    try {
      await captures.stop(id);
      captureList = await captures.list();
    } catch (e: any) {
      error = e?.message ?? 'Failed to stop capture';
    }
  }

  async function remove(id: string) {
    if (!confirm('Delete this capture record? The pcap file is preserved on disk.')) return;
    try {
      await captures.delete(id);
      captureList = await captures.list();
    } catch (e: any) {
      error = e?.message ?? 'Failed to delete capture';
    }
  }

  function ssidFor(c: Record<string, any>): string {
    return c.expand?.network_id?.ssid ?? c.network_id ?? '-';
  }
</script>

<svelte:head><title>Captures - Tala WTE</title></svelte:head>

<div class="page-header">
  <div>
    <h1 class="page-title">Packet Captures</h1>
    <p class="page-subtitle">Passive wireless and network-layer capture</p>
  </div>
  <div class="header-actions">
    <button class="btn" onclick={() => (guideOpen = true)}>Guide</button>
  </div>
</div>

<GuideModal bind:open={guideOpen} title={GUIDES.captures.title} doc={GUIDES.captures.doc} />

{#if error}
  <div class="error-toast"><span>{error}</span><button onclick={() => (error = '')}>×</button></div>
{/if}

<div class="stack">
  <section class="panel">
    <div class="panel-head">
      <h2 class="panel-title">Start New Capture</h2>
    </div>
    <div class="panel-body">
      <div class="capture-form">
        <div class="field">
          <label class="field-label" for="selectedNet">Network</label>
          <select class="input" id="selectedNet" bind:value={selectedNet}>
            {#each networkList as n}
              <option value={n.id}>{n.ssid} ({n.status})</option>
            {/each}
          </select>
        </div>
        <div class="field">
          <label class="field-label" for="layer">Layer</label>
          <select class="input" id="layer" bind:value={layer}>
            <option value="network">Network (IP layer - tshark on AP interface)</option>
            <option value="wireless">Wireless (802.11 - monitor mode interface)</option>
          </select>
        </div>
        <div class="field">
          <label class="field-label" for="iface">Interface</label>
          {#if interfaces.length}
            <select class="input" id="iface" bind:value={iface}>
              {#each interfaces as i}<option value={i.interface}>{i.interface}</option>{/each}
            </select>
          {:else}
            <input class="input" id="iface" bind:value={iface} placeholder="e.g. wlan0" />
          {/if}
        </div>
        <div class="field">
          <label class="field-label" for="filter">BPF Filter (optional)</label>
          <input
            class="input"
            id="filter"
            bind:value={filter}
            placeholder="e.g. &quot;port 80&quot; or &quot;host 10.0.0.1&quot;"
          />
          <div class="presets">
            {#each FILTER_PRESETS as p}
              <button
                type="button"
                class="chip preset-chip"
                class:active={filter === p.bpf && p.bpf !== ''}
                onclick={() => (filter = p.bpf)}>{p.label}</button
              >
            {/each}
          </div>
        </div>
      </div>
      <div class="form-foot">
        <button
          class="btn btn-primary"
          onclick={start}
          disabled={starting || !selectedNet || !iface}
        >
          {starting ? 'Starting…' : 'Start Capture'}
        </button>
      </div>
    </div>
  </section>

  <section class="panel">
    <div class="panel-head">
      <h2 class="panel-title">Capture Sessions</h2>
      {#if !loading}<span class="count-pill">{captureList.length}</span>{/if}
    </div>
    {#if loading}
      <div class="empty-state"><p>Loading capture sessions…</p></div>
    {:else if captureList.length === 0}
      <div class="empty-state">
        <p>No capture sessions yet</p>
        <p class="dim">Start a capture above to begin recording traffic.</p>
      </div>
    {:else}
      <div class="table-wrap">
        <table class="table">
          <thead>
            <tr>
              <th>SSID</th><th>Layer</th><th>Interface</th>
              <th class="num">Packets</th><th>Status</th><th class="act"></th>
            </tr>
          </thead>
          <tbody>
            {#each captureList as c}
              <tr>
                <td data-label="SSID" class="mono">{ssidFor(c)}</td>
                <td data-label="Layer"><span class="badge badge-neutral">{c.layer}</span></td>
                <td data-label="Interface" class="mono dim">{c.interface}</td>
                <td data-label="Packets" class="mono num">{c.packet_count ?? '-'}</td>
                <td data-label="Status">
                  <span class="status-label">
                    <span
                      class="status-dot"
                      class:active={c.status === 'running'}
                      class:inactive={c.status !== 'running'}
                    ></span>
                    {c.status}
                  </span>
                </td>
                <td data-label="" class="act">
                  <div class="row-actions">
                    {#if c.status === 'running'}
                      <button class="action-btn btn-danger" onclick={() => stop(c.id)}>Stop</button>
                    {:else}
                      <a href={`/captures/${c.id}`} class="action-btn">View</a>
                      <a href={captures.downloadURL(c.id)} class="action-btn" download>Download</a>
                      <button class="action-btn btn-danger" onclick={() => remove(c.id)}>Del</button
                      >
                    {/if}
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </section>
</div>

<style>
  .capture-form {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: var(--space-lg);
  }
  .form-foot {
    display: flex;
    margin-top: var(--space-lg);
    padding-top: var(--space-lg);
    border-top: 1px solid var(--border-subtle);
  }
  .status-label {
    display: inline-flex;
    align-items: center;
    text-transform: capitalize;
    white-space: nowrap;
  }
  .table .num {
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
  .table th.act,
  .table td.act {
    text-align: right;
  }
  .btn-danger.action-btn {
    color: var(--color-red);
    border-color: rgba(244, 63, 94, 0.4);
    background: rgba(244, 63, 94, 0.08);
  }
  .btn-danger.action-btn:hover {
    background: var(--color-red);
    color: #fff;
    border-color: transparent;
  }

  .presets {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-top: var(--space-sm);
  }
  .preset-chip {
    font-size: var(--font-size-xs);
    padding: 3px 10px;
    cursor: pointer;
  }
</style>
