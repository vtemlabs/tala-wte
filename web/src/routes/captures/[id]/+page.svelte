<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government
  use require a license from VTEM Labs. The Software may not be copied or
  redistributed. See the LICENSE file.
-->
<script lang="ts">
  import { page } from '$app/state';
  import { onMount } from 'svelte';
  import { captures } from '$lib/api';
  import { toast } from '$lib/stores/toast';

  const id = $derived(page.params.id ?? '');

  let rec = $state<Record<string, any> | null>(null);
  let analysis = $state<Record<string, any> | null>(null);
  let analyzing = $state(true);

  let packets = $state<Record<string, any>[]>([]);
  let truncated = $state(false);
  let loadingPackets = $state(false);
  let displayFilter = $state('');

  let selectedNo = $state(0);
  let detail = $state('');
  let loadingDetail = $state(false);

  let tab = $state<'analysis' | 'packets'>('analysis');

  const ssid = $derived(rec?.expand?.network_id?.ssid ?? rec?.network_id ?? 'Capture');

  onMount(async () => {
    try {
      rec = await captures.get(id);
    } catch {
      /* record optional for view */
    }
    loadAnalysis();
    loadPackets();
  });

  async function loadAnalysis() {
    analyzing = true;
    try {
      analysis = await captures.analyze(id);
    } catch (e: any) {
      toast.err(e?.message ?? 'Analysis failed');
    }
    analyzing = false;
  }

  async function loadPackets() {
    loadingPackets = true;
    try {
      const r = await captures.packets(id, displayFilter, 1000);
      packets = r.packets ?? [];
      truncated = r.truncated ?? false;
    } catch (e: any) {
      toast.err(e?.message ?? 'Display filter error');
    }
    loadingPackets = false;
  }

  async function selectPacket(n: number) {
    selectedNo = n;
    loadingDetail = true;
    detail = '';
    try {
      const r = await captures.packetDetail(id, n);
      detail = r.detail ?? '';
    } catch {
      detail = 'Could not load packet detail.';
    }
    loadingDetail = false;
  }
</script>

<svelte:head><title>{ssid} - Capture Viewer - Tala WTE</title></svelte:head>

<div class="page-header">
  <div>
    <a class="crumb" href="/captures">Captures</a>
    <h1 class="page-title">{ssid}</h1>
    <p class="page-subtitle">
      {#if rec}{rec.layer} layer on {rec.interface}{#if rec.filter}
          &middot; filter "{rec.filter}"{/if}{/if}
    </p>
  </div>
  <div class="header-actions">
    <a href={captures.downloadURL(id)} class="btn btn-primary" download>Download pcap</a>
    <a href="/captures" class="btn">Back</a>
  </div>
</div>

<div class="tab-bar">
  <button class="tab" class:active={tab === 'analysis'} onclick={() => (tab = 'analysis')}
    >Analysis</button
  >
  <button class="tab" class:active={tab === 'packets'} onclick={() => (tab = 'packets')}>
    Packets{#if packets.length}<span class="count-pill">{packets.length}{truncated ? '+' : ''}</span
      >{/if}
  </button>
</div>

{#if tab === 'analysis'}
  {#if analyzing}
    <div class="empty-state"><p>Analyzing capture...</p></div>
  {:else if analysis}
    <div class="stat-strip panel an-strip">
      <div class="stat-cell">
        <span class="k">Packets</span><span class="v">{analysis.packets}</span>
      </div>
      <div class="stat-cell">
        <span class="k">Duration</span><span class="v"
          >{analysis.duration_sec ? analysis.duration_sec.toFixed(1) + 's' : '-'}</span
        >
      </div>
      <div class="stat-cell">
        <span class="k">Size</span><span class="v">{analysis.file_size_mb?.toFixed(2)} MB</span>
      </div>
      <div class="stat-cell">
        <span class="k">Protocols</span><span class="v">{analysis.protocols?.length ?? 0}</span>
      </div>
    </div>
    {#if analysis.note}<p class="an-note">{analysis.note}</p>{/if}

    <div class="grid grid-2" style="align-items:start">
      {#if analysis.credentials?.length}
        <section class="panel span-2">
          <div class="panel-head">
            <h2 class="panel-title danger">Cleartext credentials</h2>
            <span class="count-pill">{analysis.credentials.length}</span>
          </div>
          <div class="table-wrap">
            <table class="table">
              <thead><tr><th>Type</th><th>Host</th><th>Captured</th></tr></thead>
              <tbody
                >{#each analysis.credentials as cr}<tr
                    ><td data-label="Type">{cr.kind}</td><td data-label="Host" class="mono dim"
                      >{cr.source || '-'}</td
                    ><td data-label="Captured" class="mono secret">{cr.detail}</td></tr
                  >{/each}</tbody
              >
            </table>
          </div>
        </section>
      {/if}

      {#if analysis.protocols?.length}
        <section class="panel">
          <div class="panel-head"><h2 class="panel-title">Protocol mix</h2></div>
          <div class="panel-body">
            {#each analysis.protocols.slice(0, 12) as p}
              <div class="proto-row">
                <span class="proto-name mono">{p.protocol}</span>
                <span class="proto-bar"
                  ><span
                    class="proto-fill"
                    style="width:{analysis.packets
                      ? Math.max(2, (p.packets / analysis.packets) * 100)
                      : 0}%"
                  ></span></span
                >
                <span class="proto-count mono">{p.packets}</span>
              </div>
            {/each}
          </div>
        </section>
      {/if}

      {#if analysis.top_talkers?.length}
        <section class="panel">
          <div class="panel-head"><h2 class="panel-title">Top talkers</h2></div>
          <div class="table-wrap">
            <table class="table">
              <thead
                ><tr><th>Endpoint A</th><th>Endpoint B</th><th class="num">Packets</th></tr></thead
              >
              <tbody
                >{#each analysis.top_talkers as t}<tr
                    ><td data-label="Endpoint A" class="mono">{t.a}</td><td
                      data-label="Endpoint B"
                      class="mono">{t.b}</td
                    ><td data-label="Packets" class="mono num">{t.packets}</td></tr
                  >{/each}</tbody
              >
            </table>
          </div>
        </section>
      {/if}

      {#if analysis.http_requests?.length}
        <section class="panel">
          <div class="panel-head"><h2 class="panel-title">HTTP requests</h2></div>
          <div class="table-wrap">
            <table class="table">
              <thead><tr><th>Method</th><th>Host</th><th>URI</th></tr></thead>
              <tbody
                >{#each analysis.http_requests as h}<tr
                    ><td data-label="Method" class="mono">{h.method}</td><td
                      data-label="Host"
                      class="mono dim">{h.host}</td
                    ><td data-label="URI" class="mono uri">{h.uri}</td></tr
                  >{/each}</tbody
              >
            </table>
          </div>
        </section>
      {/if}

      {#if analysis.tls_server_names?.length}
        <section class="panel">
          <div class="panel-head"><h2 class="panel-title">TLS server names (SNI)</h2></div>
          <div class="panel-body">
            <div class="chip-list">
              {#each analysis.tls_server_names as s}<span class="chip-item mono"
                  >{s.value}<em>{s.count}</em></span
                >{/each}
            </div>
          </div>
        </section>
      {/if}

      {#if analysis.dns_queries?.length}
        <section class="panel">
          <div class="panel-head"><h2 class="panel-title">DNS queries</h2></div>
          <div class="panel-body">
            <div class="chip-list">
              {#each analysis.dns_queries as d}<span class="chip-item mono"
                  >{d.value}<em>{d.count}</em></span
                >{/each}
            </div>
          </div>
        </section>
      {/if}

      {#if analysis.user_agents?.length}
        <section class="panel span-2">
          <div class="panel-head"><h2 class="panel-title">HTTP user agents</h2></div>
          <div class="panel-body">
            <div class="ua-list">
              {#each analysis.user_agents as u}<div class="ua-item mono">{u.value}</div>{/each}
            </div>
          </div>
        </section>
      {/if}
    </div>

    {#if !analysis.protocols?.length && !analysis.credentials?.length && !analysis.top_talkers?.length}
      <div class="empty-state">
        <p>
          No analyzable traffic in this capture. It may be empty or contain only link-layer frames.
        </p>
      </div>
    {/if}
  {/if}
{:else}
  <section class="panel">
    <div class="panel-head">
      <form
        class="pf-form"
        onsubmit={(e) => {
          e.preventDefault();
          loadPackets();
        }}
      >
        <input
          class="input pf-input"
          bind:value={displayFilter}
          placeholder="Display filter, e.g. http or ip.addr==10.0.0.1 or dns"
        />
        <button type="submit" class="btn btn-primary">Apply</button>
        {#if displayFilter}<button
            type="button"
            class="btn"
            onclick={() => {
              displayFilter = '';
              loadPackets();
            }}>Clear</button
          >{/if}
      </form>
      {#if truncated}<span class="trunc">showing first {packets.length}</span>{/if}
    </div>
    {#if loadingPackets}
      <div class="empty-state"><p>Loading packets...</p></div>
    {:else if packets.length === 0}
      <div class="empty-state"><p>No packets match.</p></div>
    {:else}
      <div class="pk-wrap">
        <table class="table pk-table">
          <thead
            ><tr
              ><th class="num">No.</th><th class="num">Time</th><th>Source</th><th>Destination</th
              ><th>Protocol</th><th class="num">Len</th><th>Info</th></tr
            ></thead
          >
          <tbody>
            {#each packets as p}
              <tr class="pk-row" class:sel={selectedNo === p.no} onclick={() => selectPacket(p.no)}>
                <td data-label="No." class="num mono dim">{p.no}</td>
                <td data-label="Time" class="num mono dim">{Number(p.time).toFixed(3)}</td>
                <td data-label="Source" class="mono">{p.source}</td>
                <td data-label="Destination" class="mono">{p.dest}</td>
                <td data-label="Protocol"><span class="badge badge-neutral">{p.protocol}</span></td>
                <td data-label="Len" class="num mono dim">{p.length}</td>
                <td data-label="Info" class="mono info">{p.info}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </section>

  {#if selectedNo}
    <section class="panel pk-detail">
      <div class="panel-head"><h2 class="panel-title">Frame {selectedNo}</h2></div>
      <div class="panel-body">
        {#if loadingDetail}<p class="dim">Loading dissection...</p>{:else}<pre
            class="detail-text">{detail}</pre>{/if}
      </div>
    </section>
  {/if}
{/if}

<style>
  .crumb {
    font-size: var(--font-size-xs);
    color: var(--text-dim);
  }
  .crumb:hover {
    color: var(--accent-hover);
  }

  .tab-bar {
    display: flex;
    gap: 4px;
    border-bottom: 1px solid var(--border-primary);
    margin-bottom: var(--space-lg);
  }
  .tab {
    background: none;
    border: none;
    cursor: pointer;
    padding: 9px 16px;
    font-size: var(--font-size-sm);
    font-weight: 600;
    color: var(--text-muted);
    border-bottom: 2px solid transparent;
    display: inline-flex;
    align-items: center;
    gap: 7px;
  }
  .tab:hover {
    color: var(--text-secondary);
  }
  .tab.active {
    color: var(--text-primary);
    border-bottom-color: var(--accent);
  }

  .an-strip {
    margin-bottom: var(--space-lg);
  }
  .an-note {
    color: var(--text-dim);
    font-size: var(--font-size-xs);
    margin-bottom: var(--space-md);
  }
  .span-2 {
    grid-column: 1 / -1;
  }
  .panel-title.danger {
    color: var(--color-red);
  }
  .secret {
    color: var(--color-red);
    font-weight: 700;
  }
  .uri {
    max-width: 320px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .proto-row {
    display: flex;
    align-items: center;
    gap: var(--space-md);
    margin-bottom: 5px;
  }
  .proto-name {
    width: 130px;
    flex-shrink: 0;
    color: var(--text-secondary);
    font-size: var(--font-size-xs);
  }
  .proto-bar {
    flex: 1;
    height: 8px;
    background: var(--bg-input);
    border-radius: 4px;
    overflow: hidden;
  }
  .proto-fill {
    display: block;
    height: 100%;
    background: var(--accent);
    border-radius: 4px;
  }
  .proto-count {
    width: 60px;
    flex-shrink: 0;
    text-align: right;
    color: var(--text-muted);
    font-size: var(--font-size-xs);
  }

  .chip-list {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  .chip-item {
    font-size: var(--font-size-xs);
    background: var(--bg-input);
    border: 1px solid var(--border-subtle);
    border-radius: var(--radius-sm);
    padding: 3px 9px;
    color: var(--text-secondary);
  }
  .chip-item em {
    color: var(--accent-hover);
    font-style: normal;
    margin-left: 6px;
  }
  .ua-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .ua-item {
    font-size: var(--font-size-xs);
    color: var(--text-secondary);
    word-break: break-all;
  }

  .pf-form {
    display: flex;
    gap: var(--space-sm);
    flex: 1;
    max-width: 640px;
  }
  .pf-input {
    flex: 1;
    font-family: var(--font-mono);
  }
  .trunc {
    font-size: var(--font-size-xs);
    color: var(--color-yellow);
  }

  .pk-wrap {
    max-height: 560px;
    overflow-y: auto;
  }
  .pk-table {
    font-size: var(--font-size-xs);
  }
  .pk-table .num {
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
  .pk-row {
    cursor: pointer;
  }
  .pk-row:hover {
    background: var(--bg-hover);
  }
  .pk-row.sel {
    background: var(--accent-soft);
  }
  .info {
    max-width: 480px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .pk-detail {
    margin-top: var(--space-lg);
  }
  .detail-text {
    margin: 0;
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    line-height: 1.5;
    color: var(--text-secondary);
    white-space: pre-wrap;
    word-break: break-word;
    max-height: 520px;
    overflow-y: auto;
  }
</style>
