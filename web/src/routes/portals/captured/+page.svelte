<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government
  use require a license from VTEM Labs. The Software may not be copied or
  redistributed. See the LICENSE file.
-->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { browser } from '$app/environment';
  import { submissions } from '$lib/api';
  import { toast } from '$lib/stores/toast';

  let list = $state<Record<string, any>[]>([]);
  let loading = $state(true);
  let unsub: (() => void) | null = null;

  let view = $state<'cards' | 'list'>(
    (browser && (localStorage.getItem('view:captured') as 'cards' | 'list')) || 'cards'
  );
  let sortKey = $state((browser && localStorage.getItem('sort:captured')) || 'newest');
  $effect(() => {
    if (browser) localStorage.setItem('view:captured', view);
  });
  $effect(() => {
    if (browser) localStorage.setItem('sort:captured', sortKey);
  });

  function dataOf(rec: Record<string, any>): Record<string, any> {
    try {
      return JSON.parse(rec.data_json || '{}');
    } catch {
      return {};
    }
  }
  // Control fields are shown as badges (result, source) or columns, not as raw
  // key/value rows, so the cards read like real harvested data, not the database.
  const CONTROL_FIELDS = new Set(['pack_member', 'auth_result', 'auth_user', 'accept', 'redirect']);
  function fields(rec: Record<string, any>): [string, string][] {
    return Object.entries(dataOf(rec))
      .filter(([k]) => !k.startsWith('_') && !CONTROL_FIELDS.has(norm(k)))
      .map(([k, v]) => [k, String(v)]);
  }

  const isSecret = (k: string) => /pass|pwd|secret|pin|code|cvv|card/i.test(k);
  const fmtTime = (s: string) => (s ? new Date(s).toLocaleString() : '');
  const netName = (rec: Record<string, any>) =>
    rec.expand?.network_id?.ssid || rec.network_ssid || 'unknown';

  // Normalize varying portal field names into stable list-view columns.
  const norm = (k: string) => k.toLowerCase().replace(/^_+/, '');
  function primaryUser(rec: Record<string, any>): string {
    const d = dataOf(rec);
    for (const want of ['username', 'user', 'email', 'login', 'auth_user']) {
      for (const k of Object.keys(d)) if (norm(k) === want) return String(d[k]);
    }
    return '';
  }
  function primarySecret(rec: Record<string, any>): string {
    for (const [k, v] of Object.entries(dataOf(rec))) if (isSecret(k)) return String(v);
    return '';
  }
  function authResult(rec: Record<string, any>): string {
    const d = dataOf(rec);
    for (const k of Object.keys(d)) if (norm(k) === 'auth_result') return String(d[k]);
    return '';
  }
  // packMember returns the member hostname when this submission came from a pack
  // member's traffic generator, or '' for a real target.
  function packMember(rec: Record<string, any>): string {
    const d = dataOf(rec);
    for (const k of Object.keys(d)) if (norm(k) === 'pack_member') return String(d[k]);
    return '';
  }

  const ts = (r: Record<string, any>) => (r.created ? new Date(r.created).getTime() : 0);

  const sorted = $derived.by(() => {
    const arr = [...list];
    switch (sortKey) {
      case 'oldest':
        return arr.sort((a, b) => ts(a) - ts(b));
      case 'network':
        return arr.sort((a, b) => netName(a).localeCompare(netName(b)));
      case 'user':
        return arr.sort((a, b) => primaryUser(a).localeCompare(primaryUser(b)));
      case 'result':
        return arr.sort((a, b) => authResult(a).localeCompare(authResult(b)));
      default:
        return arr.sort((a, b) => ts(b) - ts(a));
    }
  });

  async function load() {
    loading = true;
    try {
      list = await submissions.list();
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to load captured data');
    } finally {
      loading = false;
    }
  }

  onMount(async () => {
    await load();
    try {
      unsub = await submissions.subscribe((e) => {
        if (e.action === 'create') list = [e.record, ...list];
        if (e.action === 'delete') list = list.filter((r) => r.id !== e.record.id);
      });
    } catch {
      /* realtime optional */
    }
  });

  onDestroy(() => {
    if (unsub) unsub();
  });

  async function del(id: string) {
    try {
      await submissions.delete(id);
      list = list.filter((r) => r.id !== id);
    } catch (e: any) {
      toast.err(e?.message ?? 'Delete failed');
    }
  }

  async function clearAll() {
    if (!confirm(`Delete all ${list.length} captured submissions?`)) return;
    const ids = list.map((r) => r.id);
    for (const id of ids) {
      try {
        await submissions.delete(id);
      } catch {
        /* continue */
      }
    }
    list = [];
    toast.success('Captured data cleared');
  }
</script>

<svelte:head><title>Captured Data - Tala WTE</title></svelte:head>

<div class="page-header">
  <div>
    <a class="crumb" href="/portals">Portals</a>
    <h1 class="page-title">Captured Data</h1>
    <p class="page-subtitle">Credentials and PII harvested by live captive portals</p>
  </div>
  <div class="header-actions">
    <button class="btn" onclick={load}>Refresh</button>
    {#if list.length > 0}<button class="btn btn-danger" onclick={clearAll}>Clear All</button>{/if}
  </div>
</div>

{#if !loading && list.length > 0}
  <div class="stat-strip panel cap-strip">
    <div class="stat-cell">
      <span class="k">Total Submissions</span>
      <span class="v">{list.length}</span>
    </div>
    <div class="stat-cell">
      <span class="k">Distinct Networks</span>
      <span class="v">{new Set(list.map(netName)).size}</span>
    </div>
    <div class="stat-cell">
      <span class="k">Latest</span>
      <span class="v latest">{fmtTime(list[0]?.created) || '-'}</span>
    </div>
  </div>
{/if}

{#if loading}
  <div class="empty-state"><p>Loading...</p></div>
{:else if list.length === 0}
  <div class="panel cap-empty">
    <div class="empty-glyph" aria-hidden="true">
      <span class="ring"></span>
      <span class="ring r2"></span>
      <span class="dot"></span>
    </div>
    <h2>No data captured yet</h2>
    <p>
      Start an <b>Open</b> network with a portal that has form fields, connect a client, and submit the
      form. Captured credentials and PII will stream in here in real time.
    </p>
  </div>
{:else}
  <div class="cap-controls">
    <div class="seg" role="tablist" aria-label="View mode">
      <button class="seg-btn" class:active={view === 'cards'} onclick={() => (view = 'cards')}
        >Cards</button
      >
      <button class="seg-btn" class:active={view === 'list'} onclick={() => (view = 'list')}
        >List</button
      >
    </div>
    <label class="sort-wrap">
      <span class="sort-lbl">Sort</span>
      <select class="input sort-select" bind:value={sortKey}>
        <option value="newest">Newest first</option>
        <option value="oldest">Oldest first</option>
        <option value="network">Network</option>
        <option value="user">Username</option>
        <option value="result">Result</option>
      </select>
    </label>
  </div>

  {#if view === 'list'}
    <div class="panel">
      <div class="table-wrap">
        <table class="table cap-list">
          <thead>
            <tr>
              <th>Network</th><th>Captured</th><th>Username</th><th>Password</th>
              <th>Result</th><th>Source</th><th>MAC</th><th>IP</th><th class="act"></th>
            </tr>
          </thead>
          <tbody>
            {#each sorted as rec (rec.id)}
              <tr>
                <td><span class="badge badge-info">{netName(rec)}</span></td>
                <td class="mono dim">{fmtTime(rec.created) || '-'}</td>
                <td class="mono">{primaryUser(rec) || '-'}</td>
                <td class="mono secret">{primarySecret(rec) || '-'}</td>
                <td>
                  {#if authResult(rec)}
                    <span
                      class="badge {authResult(rec).toLowerCase() === 'success'
                        ? 'badge-success'
                        : 'badge-error'}">{authResult(rec)}</span
                    >
                  {:else}<span class="dim">-</span>{/if}
                </td>
                <td>
                  {#if packMember(rec)}<span class="badge badge-neutral" title="Pack member: {packMember(rec)}">pack member</span>{:else}<span class="dim">target</span>{/if}
                </td>
                <td class="mono dim">{rec.mac || '-'}</td>
                <td class="mono dim">{rec.ip || '-'}</td>
                <td class="act"
                  ><button class="action-btn cap-del" onclick={() => del(rec.id)}>Del</button></td
                >
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </div>
  {:else}
    <div class="grid grid-2 cap-grid">
      {#each sorted as rec (rec.id)}
        <div class="card cap-card">
          <header class="cap-head">
            <span class="badge badge-info net-badge" title={netName(rec)}>{netName(rec)}</span>
            {#if packMember(rec)}<span class="badge badge-neutral" title="Pack member: {packMember(rec)}">pack member</span>{/if}
            {#if authResult(rec)}<span
                class="badge {authResult(rec).toLowerCase() === 'success'
                  ? 'badge-success'
                  : 'badge-error'}">{authResult(rec)}</span
              >{/if}
            <time class="cap-time">{fmtTime(rec.created)}</time>
            <button class="action-btn cap-del" onclick={() => del(rec.id)}>Del</button>
          </header>

          <div class="cap-table">
            {#each fields(rec) as [k, v]}
              <div class="cap-row">
                <div class="cap-key">{k}</div>
                <div class="cap-val" class:secret={isSecret(k)}>{v}</div>
              </div>
            {/each}
          </div>

          <footer class="cap-foot">
            <div class="cap-meta">
              <span class="m"><i>MAC</i><b>{rec.mac || '-'}</b></span>
              <span class="m"><i>IP</i><b>{rec.ip || '-'}</b></span>
            </div>
            {#if rec.user_agent}
              <div class="cap-ua" title={rec.user_agent}>{rec.user_agent}</div>
            {/if}
          </footer>
        </div>
      {/each}
    </div>
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

  .cap-strip {
    margin-bottom: var(--space-lg);
  }
  .cap-strip .v.latest {
    font-size: var(--font-size-md);
    font-family: var(--font-mono);
  }

  .cap-controls {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: var(--space-md);
    margin-bottom: var(--space-lg);
  }
  .seg {
    display: inline-flex;
    border: 1px solid var(--border-primary);
    border-radius: var(--radius-md);
    overflow: hidden;
    background: var(--bg-card);
  }
  .seg-btn {
    padding: 7px 18px;
    font-size: var(--font-size-xs);
    font-weight: 600;
    color: var(--text-muted);
    background: transparent;
    border: none;
    cursor: pointer;
    border-right: 1px solid var(--border-primary);
    transition:
      color 0.12s,
      background 0.12s;
  }
  .seg-btn:last-child {
    border-right: none;
  }
  .seg-btn:hover {
    color: var(--text-secondary);
  }
  .seg-btn.active {
    background: var(--accent-soft);
    color: var(--accent-hover);
  }
  .sort-wrap {
    display: inline-flex;
    align-items: center;
    gap: var(--space-sm);
  }
  .sort-lbl {
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-muted);
  }
  .sort-select {
    width: auto;
    min-width: 150px;
  }

  .cap-list td.secret {
    color: var(--color-red);
    font-weight: 700;
  }
  .cap-list .dim {
    color: var(--text-dim);
  }
  .cap-list th.act,
  .cap-list td.act {
    text-align: right;
  }

  .cap-grid {
    align-items: start;
  }
  .cap-card {
    display: flex;
    flex-direction: column;
    gap: var(--space-md);
    padding: var(--space-lg);
  }

  .cap-head {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
    min-width: 0;
  }
  .net-badge {
    max-width: 45%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .cap-time {
    flex: 1;
    min-width: 0;
    font-size: var(--font-size-xs);
    color: var(--text-dim);
    font-family: var(--font-mono);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .cap-del {
    flex-shrink: 0;
    color: var(--color-red);
    border-color: rgba(244, 63, 94, 0.3);
  }
  .cap-del:hover {
    background: var(--color-red);
    color: #fff;
    border-color: transparent;
  }

  .cap-table {
    border: 1px solid var(--border-primary);
    border-radius: var(--radius-sm);
    overflow: hidden;
  }
  .cap-row {
    display: grid;
    grid-template-columns: 120px 1fr;
    border-bottom: 1px solid var(--border-subtle);
  }
  .cap-row:last-child {
    border-bottom: none;
  }
  .cap-row:hover {
    background: var(--bg-hover);
  }
  .cap-key {
    padding: var(--space-sm) var(--space-md);
    font-size: var(--font-size-xs);
    font-weight: 600;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    background: var(--bg-tertiary);
    font-family: var(--font-mono);
    word-break: break-all;
    border-right: 1px solid var(--border-primary);
  }
  .cap-val {
    padding: var(--space-sm) var(--space-md);
    font-size: var(--font-size-sm);
    color: var(--text-primary);
    font-family: var(--font-mono);
    word-break: break-all;
  }
  .cap-val.secret {
    color: var(--color-red);
    font-weight: 700;
  }

  .cap-foot {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding-top: 2px;
    border-top: 1px solid var(--border-subtle);
  }
  .cap-meta {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-md);
    padding-top: var(--space-sm);
  }
  .cap-meta .m {
    display: inline-flex;
    align-items: baseline;
    gap: 6px;
    font-size: var(--font-size-xs);
  }
  .cap-meta i {
    font-style: normal;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    font-size: 10px;
    font-weight: 700;
  }
  .cap-meta b {
    color: var(--text-secondary);
    font-family: var(--font-mono);
    font-weight: 500;
    word-break: break-all;
  }
  .cap-ua {
    font-size: 10px;
    color: var(--text-dim);
    font-family: var(--font-mono);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .cap-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    padding: var(--space-3xl) var(--space-xl);
  }
  .empty-glyph {
    position: relative;
    width: 72px;
    height: 72px;
    margin-bottom: var(--space-lg);
  }
  .empty-glyph .ring,
  .empty-glyph .dot {
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    border-radius: 50%;
  }
  .empty-glyph .ring {
    width: 72px;
    height: 72px;
    border: 1px solid var(--border-secondary);
  }
  .empty-glyph .ring.r2 {
    width: 46px;
    height: 46px;
    border-color: var(--accent);
    opacity: 0.5;
  }
  .empty-glyph .dot {
    width: 10px;
    height: 10px;
    background: var(--accent);
    box-shadow: 0 0 12px var(--accent-glow);
  }
  .cap-empty h2 {
    font-size: var(--font-size-md);
    font-weight: 700;
    color: var(--text-secondary);
  }
  .cap-empty p {
    font-size: var(--font-size-sm);
    color: var(--text-muted);
    margin-top: var(--space-sm);
    max-width: 480px;
    line-height: 1.6;
  }
  .cap-empty b {
    color: var(--text-secondary);
  }
</style>
