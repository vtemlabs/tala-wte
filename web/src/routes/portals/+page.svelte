<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government
  use require a license from VTEM Labs. The Software may not be copied or
  redistributed. See the LICENSE file.
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { browser } from '$app/environment';
  import { goto } from '$app/navigation';
  import { portals, submissions } from '$lib/api';
  import { toast } from '$lib/stores/toast';
  import GuideModal from '$lib/components/GuideModal.svelte';
  import { GUIDES } from '$lib/guides';

  let guideOpen = $state(false);
  let list = $state<Record<string, any>[]>([]);
  let loading = $state(true);
  let activeCat = $state('all');
  let submissionCount = $state(0);
  let restoring = $state(false);

  // Search, source (built-in vs custom), and sort. Source + sort persist so the
  // view is remembered across refreshes.
  let search = $state('');
  let source = $state((browser && localStorage.getItem('portals:source')) || 'all');
  let sortBy = $state((browser && localStorage.getItem('portals:sort')) || 'name');
  $effect(() => {
    if (browser) {
      localStorage.setItem('portals:source', source);
      localStorage.setItem('portals:sort', sortBy);
    }
  });

  let showUpload = $state(false);
  let uploadName = $state('');
  let uploadFile = $state<File | null>(null);
  let uploading = $state(false);

  let showScrape = $state(false);
  let scrapeURL = $state('');
  let scrapeName = $state('');
  let scraping = $state(false);

  const CATS: Record<string, string> = {
    coffee: 'Coffee',
    restaurant: 'Restaurant',
    retail: 'Retail',
    hotel: 'Hotel',
    corporate: 'Corporate Guest',
    airport: 'Airport',
    inflight: 'In-Flight',
    transit: 'Transit',
    isp: 'ISP / Carrier',
    telecom: 'Mobile Carrier',
    education: 'Education',
    healthcare: 'Healthcare',
    library: 'Library',
    event: 'Events & Venues',
    cruise: 'Cruise & Maritime',
    fitness: 'Fitness',
    automotive: 'Automotive',
    social: 'Social Login',
    generic: 'Generic',
    custom: 'Custom'
  };

  const catLabel = (c: string) => CATS[c] ?? (c || 'Uncategorized');

  let categories = $derived([
    'all',
    ...Array.from(new Set(list.map((p) => p.category || 'custom')))
  ]);
  let filtered = $derived.by(() => {
    const q = search.trim().toLowerCase();
    let out = list.filter((p) => {
      if (activeCat !== 'all' && (p.category || 'custom') !== activeCat) return false;
      if (source === 'builtin' && p.type !== 'builtin') return false;
      if (source === 'custom' && p.type === 'builtin') return false;
      if (q && !(`${p.name} ${p.description ?? ''}`.toLowerCase().includes(q))) return false;
      return true;
    });
    out = [...out].sort((a, b) => {
      if (sortBy === 'category')
        return catLabel(a.category || 'custom').localeCompare(catLabel(b.category || 'custom'));
      if (sortBy === 'type') return (a.type || '').localeCompare(b.type || '');
      return (a.name || '').localeCompare(b.name || '');
    });
    return out;
  });

  const builtinCount = $derived(list.filter((p) => p.type === 'builtin').length);
  const customCount = $derived(list.filter((p) => p.type !== 'builtin').length);

  async function restoreTemplates() {
    restoring = true;
    try {
      const res: any = await portals.restore();
      const n = res?.restored ?? 0;
      const r = res?.reset ?? 0;
      toast.success(
        n || r ? `Restored ${n} template(s), reset ${r} to original` : 'Templates already up to date'
      );
      await load();
    } catch (e: any) {
      toast.err(e?.message ?? 'Restore failed');
    } finally {
      restoring = false;
    }
  }

  async function load() {
    loading = true;
    try {
      list = await portals.list();
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to load portals');
    } finally {
      loading = false;
    }
    try {
      submissionCount = (await submissions.list()).length;
    } catch {
      /* optional */
    }
  }

  onMount(load);

  async function clone(p: Record<string, any>) {
    try {
      const rec = await portals.create({
        name: `${p.name} (copy)`,
        html: p.html,
        category: p.category || 'custom'
      });
      toast.success('Portal cloned');
      goto(`/portals/${rec.id}`);
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to clone portal');
    }
  }

  async function deletePortal(id: string, name: string) {
    if (!confirm(`Delete portal "${name}"?`)) return;
    try {
      await portals.delete(id);
      list = list.filter((p) => p.id !== id);
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to delete portal');
    }
  }

  function onFile(e: Event) {
    const f = (e.target as HTMLInputElement).files?.[0] ?? null;
    uploadFile = f;
    if (f && !uploadName.trim()) uploadName = f.name.replace(/\.(html?|zip)$/i, '');
  }

  async function doUpload() {
    if (!uploadFile) {
      toast.err('Choose a .html or .zip file');
      return;
    }
    if (!uploadName.trim()) {
      toast.err('Give the portal a name');
      return;
    }
    uploading = true;
    try {
      const rec = await portals.upload(uploadFile, uploadName.trim());
      toast.success('Portal uploaded');
      goto(`/portals/${rec.id}`);
    } catch (e: any) {
      toast.err(e?.message ?? 'Upload failed');
    } finally {
      uploading = false;
    }
  }

  async function doScrape() {
    const u = scrapeURL.trim();
    if (!/^https?:\/\//i.test(u)) {
      toast.err('Enter a full http(s) URL');
      return;
    }
    scraping = true;
    try {
      const rec = await portals.scrape(u, scrapeName.trim());
      toast.success('Portal cloned from URL');
      goto(`/portals/${rec.id}`);
    } catch (e: any) {
      toast.err(e?.message ?? 'Clone failed');
    } finally {
      scraping = false;
    }
  }
</script>

<svelte:head><title>Captive Portals - Tala WTE</title></svelte:head>

<svelte:window
  onkeydown={(e) => {
    if (e.key !== 'Escape') return;
    if (showScrape && !scraping) showScrape = false;
    else if (showUpload && !uploading) showUpload = false;
  }}
/>

<div class="page-header">
  <div>
    <h1 class="page-title">Captive Portals</h1>
    <p class="page-subtitle">Built-in and custom portal templates for open networks</p>
  </div>
  <div class="header-actions">
    <button class="btn" onclick={() => (guideOpen = true)}>Guide</button>
    <button
      class="btn"
      onclick={() => {
        showScrape = true;
      }}>Clone from URL</button
    >
    <button
      class="btn"
      onclick={() => {
        showUpload = true;
      }}>Upload Template</button
    >
    <button class="btn" onclick={restoreTemplates} disabled={restoring}
      >{restoring ? 'Restoring...' : 'Restore Templates'}</button
    >
    <a href="/portals/new" class="btn btn-primary">+ New Portal</a>
  </div>
</div>

<a href="/portals/captured" class="panel captured-link">
  <span class="captured-tick" aria-hidden="true"></span>
  <span class="captured-text">
    <span class="captured-title">Captured Data</span>
    <span class="captured-sub">Credentials &amp; PII harvested by live portals</span>
  </span>
  <span class="count-pill captured-count" class:lit={submissionCount > 0}>{submissionCount}</span>
</a>

{#if !loading && list.length > 0}
  <div class="portal-controls">
    <input class="input filter-field" bind:value={search} placeholder="Search portals…" />
    <button class="chip" class:active={source === 'all'} onclick={() => (source = 'all')}
      >All ({list.length})</button
    >
    <button class="chip" class:active={source === 'builtin'} onclick={() => (source = 'builtin')}
      >Built-in ({builtinCount})</button
    >
    <button class="chip" class:active={source === 'custom'} onclick={() => (source = 'custom')}
      >Custom ({customCount})</button
    >
    <select class="input sort-select" bind:value={sortBy} aria-label="Sort by">
      <option value="name">Sort: Name</option>
      <option value="category">Sort: Category</option>
      <option value="type">Sort: Source</option>
    </select>
    <span class="count-pill">{filtered.length}</span>
  </div>
  <div class="filter-row">
    {#each categories as c}
      <button class="chip" class:active={activeCat === c} onclick={() => (activeCat = c)}>
        {c === 'all' ? 'All' : catLabel(c)}
      </button>
    {/each}
  </div>
{/if}

{#if loading}
  <div class="empty-state"><p>Loading portals...</p></div>
{:else if list.length === 0}
  <div class="empty-state">
    <p>No portals configured. Create one, or upload a template to get started.</p>
    <a href="/portals/new" class="btn btn-primary" style="margin-top:var(--space-lg)"
      >Create Portal</a
    >
  </div>
{:else}
  <div class="grid grid-3 gallery">
    {#each filtered as portal (portal.id)}
      <article class="card portal-card">
        <div class="thumb">
          {#if portal.html?.startsWith('fs:')}
            <iframe
              src={portals.previewURL(portal.id)}
              title={portal.name}
              loading="lazy"
              sandbox="allow-scripts allow-forms"
            ></iframe>
          {:else}
            <iframe
              srcdoc={portal.html}
              title={portal.name}
              loading="lazy"
              sandbox="allow-scripts allow-forms"
            ></iframe>
          {/if}
        </div>
        <div class="portal-head">
          <div class="portal-name">{portal.name}</div>
          <span class="badge {portal.type === 'builtin' ? 'badge-info' : 'badge-neutral'}"
            >{portal.type}</span
          >
        </div>
        <div class="portal-cat">{catLabel(portal.category || 'custom')}</div>
        {#if portal.description}
          <p class="portal-desc">{portal.description}</p>
        {/if}
        <div class="portal-actions">
          <a
            href="/portals/{portal.id}"
            class="action-btn"
            title={portal.type === 'builtin'
              ? 'Built-in templates are read-only; saving creates an editable copy'
              : 'Edit this portal'}>{portal.type === 'builtin' ? 'Customize' : 'Edit'}</a
          >
          <a href={portals.previewURL(portal.id)} target="_blank" rel="noopener" class="action-btn"
            >Preview</a
          >
          {#if !portal.html?.startsWith('fs:')}
            <button class="action-btn" onclick={() => clone(portal)}>Clone</button>
          {/if}
          <button class="action-btn danger" onclick={() => deletePortal(portal.id, portal.name)}
            >Delete</button
          >
        </div>
      </article>
    {/each}
  </div>
{/if}

{#if showScrape}
  <div
    class="overlay"
    onclick={() => {
      if (!scraping) showScrape = false;
    }}
    onkeydown={(e) => {
      if (e.key === 'Escape' && !scraping) showScrape = false;
    }}
    role="presentation"
  >
    <div
      class="upload-modal"
      onclick={(e) => e.stopPropagation()}
      role="dialog"
      aria-modal="true"
      tabindex="-1"
    >
      <div class="modal-header" style="cursor:default">
        <span class="modal-title">Clone Portal from URL</span>
        <button
          class="action-btn"
          onclick={() => {
            if (!scraping) showScrape = false;
          }}>Close</button
        >
      </div>
      <div class="modal-body" style="display:flex;flex-direction:column;gap:var(--space-lg)">
        <div class="form-group">
          <label class="field-label" for="surl">Page URL</label>
          <input
            class="input"
            id="surl"
            bind:value={scrapeURL}
            placeholder="https://example.com/login"
          />
          <div class="field-desc">
            Fetches the page, inlines its CSS &amp; images so it renders offline, drops external
            scripts, and wires its login to capture+authenticate. Private/internal addresses are
            blocked.
          </div>
        </div>
        <div class="form-group">
          <label class="field-label" for="sname"
            >Portal Name <span style="color:var(--text-dim)">(optional)</span></label
          >
          <input
            class="input"
            id="sname"
            bind:value={scrapeName}
            placeholder="defaults to the site host"
          />
        </div>
        <div class="header-actions">
          <button class="btn btn-primary" onclick={doScrape} disabled={scraping}
            >{scraping ? 'Cloning...' : 'Clone'}</button
          >
          <button
            class="btn"
            onclick={() => {
              if (!scraping) showScrape = false;
            }}>Cancel</button
          >
        </div>
      </div>
    </div>
  </div>
{/if}

{#if showUpload}
  <div
    class="overlay"
    onclick={() => {
      if (!uploading) showUpload = false;
    }}
    onkeydown={(e) => {
      if (e.key === 'Escape' && !uploading) showUpload = false;
    }}
    role="presentation"
  >
    <div
      class="upload-modal"
      onclick={(e) => e.stopPropagation()}
      role="dialog"
      aria-modal="true"
      tabindex="-1"
    >
      <div class="modal-header" style="cursor:default">
        <span class="modal-title">Upload Portal Template</span>
        <button
          class="action-btn"
          onclick={() => {
            if (!uploading) showUpload = false;
          }}>Close</button
        >
      </div>
      <div class="modal-body" style="display:flex;flex-direction:column;gap:var(--space-lg)">
        <div class="form-group">
          <label class="field-label" for="uname">Portal Name</label>
          <input
            class="input"
            id="uname"
            bind:value={uploadName}
            placeholder="e.g. Acme Guest Wi-Fi"
          />
        </div>
        <div class="form-group">
          <label class="field-label" for="ufile">Template File</label>
          <input class="input" id="ufile" type="file" accept=".html,.htm,.zip" onchange={onFile} />
          <div class="field-desc">
            Single self-contained <code>.html</code>, or a <code>.zip</code> bundle with an
            <code>index.html</code> at its root plus assets (images, css, js).
          </div>
        </div>
        <div class="header-actions">
          <button class="btn btn-primary" onclick={doUpload} disabled={uploading}
            >{uploading ? 'Uploading...' : 'Upload'}</button
          >
          <button
            class="btn"
            onclick={() => {
              if (!uploading) showUpload = false;
            }}>Cancel</button
          >
        </div>
      </div>
    </div>
  </div>
{/if}

<GuideModal bind:open={guideOpen} title={GUIDES.portals.title} doc={GUIDES.portals.doc} />

<style>
  .captured-link {
    display: flex;
    align-items: center;
    gap: var(--space-md);
    padding: var(--space-md) var(--space-xl);
    margin-bottom: var(--space-lg);
    text-decoration: none;
    transition:
      border-color var(--transition-fast),
      background var(--transition-fast);
  }
  .captured-link:hover {
    border-color: var(--accent);
    background: var(--accent-soft);
  }
  .captured-tick {
    width: 3px;
    height: 30px;
    border-radius: 2px;
    flex-shrink: 0;
    background: var(--accent);
    box-shadow: 0 0 8px var(--accent-glow);
  }
  .captured-text {
    display: flex;
    flex-direction: column;
    min-width: 0;
    flex: 1;
  }
  .captured-title {
    font-weight: 600;
    color: var(--text-primary);
  }
  .captured-sub {
    font-size: var(--font-size-xs);
    color: var(--text-muted);
    margin-top: 1px;
  }
  .captured-count {
    font-size: var(--font-size-base);
    padding: 4px 13px;
    flex-shrink: 0;
  }
  .captured-count.lit {
    color: var(--color-green);
    background: rgba(34, 197, 94, 0.14);
    border-color: rgba(34, 197, 94, 0.35);
  }

  .portal-controls {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-sm);
    margin-bottom: var(--space-md);
  }
  .filter-field {
    max-width: 280px;
    margin-right: auto;
  }
  .sort-select {
    width: auto;
  }
  .filter-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-sm);
    margin-bottom: var(--space-lg);
  }

  .gallery {
    margin-bottom: var(--space-xl);
  }
  .portal-card {
    display: flex;
    flex-direction: column;
    gap: var(--space-sm);
    padding: var(--space-md);
  }
  .portal-card:hover {
    border-color: var(--border-secondary);
  }

  .thumb {
    height: 150px;
    border: 1px solid var(--border-primary);
    border-radius: var(--radius-sm);
    background: #fff;
    overflow: hidden;
    position: relative;
  }
  .thumb iframe {
    width: 250%;
    height: 250%;
    border: none;
    pointer-events: none;
    transform: scale(0.4);
    transform-origin: 0 0;
  }

  .portal-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--space-sm);
  }
  .portal-name {
    font-weight: 600;
    color: var(--text-primary);
    line-height: 1.3;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }
  .portal-cat {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-dim);
  }
  .portal-desc {
    font-size: var(--font-size-xs);
    color: var(--text-muted);
    line-height: 1.5;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
  .portal-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    margin-top: auto;
    padding-top: var(--space-sm);
  }
  .action-btn {
    text-decoration: none;
  }
  .action-btn.danger {
    color: var(--color-red);
    border-color: rgba(244, 63, 94, 0.4);
    background: rgba(244, 63, 94, 0.08);
    margin-left: auto;
  }
  .action-btn.danger:hover {
    background: var(--color-red);
    color: #fff;
    border-color: transparent;
  }

  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    backdrop-filter: blur(2px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 400;
    padding: 1rem;
  }
  .upload-modal {
    background: rgba(20, 20, 20, 0.98);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: var(--radius-lg);
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.6);
    width: 100%;
    max-width: 460px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
</style>
