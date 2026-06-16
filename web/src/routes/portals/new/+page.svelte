<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, paid training, paid CTF,
  or any for-profit use requires a license from VTEM Labs. See the LICENSE file.
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { portals, type PortalTemplate } from '$lib/api';

  let name = $state('');
  let html = $state('');
  let saving = $state(false);
  let error = $state('');
  let templates = $state<PortalTemplate[]>([]);
  let picked = $state('');

  const defaultHTML = `<!DOCTYPE html>
<html lang="en"><head><meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0"><title>Portal</title>
<style>body{font-family:sans-serif;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;background:#f5f5f5;}
.card{background:#fff;padding:2rem;border-radius:12px;max-width:400px;width:90%;text-align:center;box-shadow:0 4px 24px rgba(0,0,0,0.1);}
button{background:#2563eb;color:#fff;border:none;padding:.75rem 2rem;border-radius:8px;cursor:pointer;width:100%;font-size:1rem;}</style>
</head><body><div class="card">
<h1>Welcome</h1><p>Accept terms to connect.</p>
<form method="POST" action="/portal/accept"><button>Connect</button></form>
</div></body></html>`;

  onMount(async () => {
    try {
      templates = await portals.templates();
    } catch {
      /* gallery is optional */
    }
    const slug = page.url.searchParams.get('template');
    if (slug) {
      const t = templates.find((x) => x.slug === slug);
      if (t) {
        picked = slug;
        html = t.html;
        if (!name) name = t.name;
      }
    }
  });

  function applyTemplate() {
    const t = templates.find((x) => x.slug === picked);
    if (t) {
      html = t.html;
      if (!name.trim()) name = t.name;
    }
  }

  async function save() {
    if (!name.trim()) {
      error = 'Name is required';
      return;
    }
    if (!html.trim()) {
      error = 'HTML content is required';
      return;
    }
    saving = true;
    error = '';
    try {
      const rec = await portals.create({ name: name.trim(), html });
      goto(`/portals/${rec.id}`);
    } catch (e: any) {
      error = e?.message ?? 'Failed to save portal';
    }
    saving = false;
  }
</script>

<svelte:head><title>New Portal - Tala WTE</title></svelte:head>

<div class="page-header">
  <div>
    <h1 class="page-title">New Captive Portal</h1>
    <p class="page-subtitle">
      Start from a built-in template, paste your own HTML, or build from scratch
    </p>
  </div>
  <a href="/portals" class="btn">Back</a>
</div>

{#if error}
  <div class="error-toast">
    <span>{error}</span><button class="action-btn" onclick={() => (error = '')}>x</button>
  </div>
{/if}

<div class="grid grid-2" style="margin-bottom:var(--space-lg);align-items:end">
  <div class="form-group">
    <label class="field-label" for="pname">Portal Name</label>
    <input class="input" id="pname" bind:value={name} placeholder="e.g. Coffee Shop Portal" />
  </div>
  <div class="form-group">
    <label class="field-label" for="ptmpl">Start From Template</label>
    <div class="header-actions">
      <select class="input" id="ptmpl" bind:value={picked} style="flex:1">
        <option value="">Blank</option>
        {#each templates as t}
          <option value={t.slug}>{t.name}</option>
        {/each}
      </select>
      <button class="btn" onclick={applyTemplate} disabled={!picked}>Load</button>
    </div>
  </div>
</div>

<div
  style="display:grid;grid-template-columns:1fr 1fr;gap:var(--space-lg);margin-bottom:var(--space-lg)"
>
  <div>
    <div class="section-title">HTML Source</div>
    <textarea
      class="input"
      style="min-height:460px;font-family:var(--font-mono);font-size:0.75rem;resize:vertical"
      bind:value={html}
      placeholder={defaultHTML}
    ></textarea>
    <div class="header-actions" style="margin-top:var(--space-sm)">
      <button class="btn btn-sm" onclick={() => (html = defaultHTML)}>Insert Starter HTML</button>
    </div>
  </div>
  <div>
    <div class="section-title">Live Preview</div>
    <div
      style="border:1px solid var(--border-primary);border-radius:var(--radius-md);overflow:hidden;height:460px;background:white"
    >
      {#if html}
        <iframe
          srcdoc={html}
          title="Portal preview"
          style="width:100%;height:100%;border:none"
          sandbox="allow-scripts allow-forms"
        ></iframe>
      {:else}
        <div class="empty-state" style="background:#f5f5f5;height:100%;color:#999">
          <p>Paste HTML or pick a template to preview</p>
        </div>
      {/if}
    </div>
  </div>
</div>

<div class="header-actions">
  <button class="btn btn-primary" onclick={save} disabled={saving}
    >{saving ? 'Saving...' : 'Save Portal'}</button
  >
  <a href="/portals" class="btn">Cancel</a>
</div>
