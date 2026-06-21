<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government
  use require a license from VTEM Labs. The Software may not be copied or
  redistributed. See the LICENSE file.
-->
<script lang="ts">
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { portals } from '$lib/api';
  import { toast } from '$lib/stores/toast';

  const id = $derived(page.params.id ?? '');
  let portal = $state<Record<string, any> | null>(null);
  // Built-in templates are read-only; editing one forks an editable custom copy.
  let isBuiltin = $derived(portal?.type === 'builtin');
  let html = $state('');
  let name = $state('');
  let authType = $state('');
  let authTypes = $state<Record<string, any>[]>([]);
  let saving = $state(false);
  let error = $state('');
  let saved = $state(false);
  let device = $state<'desktop' | 'mobile'>('desktop');
  const authLabel = (t: string) =>
    authTypes.find((a) => a.type === t)?.label ?? (t || 'Click-through');

  let isBundle = $derived(html.startsWith('fs:'));

  // Parse field name attributes to show which fields this portal harvests.
  let detected = $derived.by(() => {
    if (isBundle) return [];
    const out = new Set<string>();
    const re = /<(?:input|select|textarea)\b[^>]*?\bname=["']([^"']+)["']/gi;
    let m;
    while ((m = re.exec(html)) !== null) {
      if (m[1] !== 'redirect') out.add(m[1]);
    }
    return Array.from(out);
  });

  onMount(async () => {
    try {
      portal = await portals.get(id);
      html = portal.html;
      name = portal.name;
      authType = portal.auth_type || '';
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to load portal');
    }
    try {
      authTypes = await portals.authTypes();
    } catch {
      /* optional */
    }
  });

  async function save() {
    saving = true;
    error = '';
    saved = false;
    try {
      if (isBuiltin) {
        // A built-in template is never modified in place; saving forks a custom copy.
        const copyName = /\(copy\)\s*$/i.test(name) ? name : `${name} (copy)`;
        const rec = await portals.create({
          name: copyName,
          html,
          category: portal?.category || 'custom',
          auth_type: authType
        });
        toast.success('Saved as a new copy (built-in templates are read-only)');
        goto(`/portals/${rec.id}`);
        return;
      }
      await portals.update(id, { name, html, auth_type: authType });
      saved = true;
      setTimeout(() => (saved = false), 3000);
    } catch (e: any) {
      error = e?.message ?? 'Save failed';
    }
    saving = false;
  }
</script>

<svelte:head><title>{portal?.name ?? 'Portal'} - Tala WTE</title></svelte:head>

<div class="editor">
  <header class="editor-head">
    <div class="head-id">
      <a class="crumb" href="/portals">Portals</a>
      <input
        class="input name-input"
        id="pname"
        aria-label="Portal name"
        placeholder="Portal name"
        bind:value={name}
      />
    </div>
    <div class="head-actions">
      {#if saved}<span class="badge badge-success">Saved</span>{/if}
      {#if isBuiltin}<span class="badge badge-info" title="Built-in templates are read-only"
          >read-only template</span
        >{/if}
      {#if isBuiltin}
        <span class="badge badge-neutral" title="Captive-portal auth type"
          >{authLabel(authType)}</span
        >
      {:else if authTypes.length}
        <select
          class="input auth-select"
          bind:value={authType}
          aria-label="Auth type"
          title="Captive-portal auth type"
        >
          {#each authTypes as a}<option value={a.type}>{a.label}</option>{/each}
        </select>
      {/if}
      <a href={portals.previewURL(id)} target="_blank" rel="noopener" class="btn">Open Preview</a>
      <a href="/portals" class="btn">Back</a>
      <button class="btn btn-primary" onclick={save} disabled={saving}
        >{saving ? 'Saving...' : isBuiltin ? 'Save as Copy' : 'Save Changes'}</button
      >
    </div>
  </header>

  {#if error}
    <div class="error-toast"><span>{error}</span></div>
  {/if}

  {#if isBundle}
    <div class="bundle-note">
      This is a multi-file bundle (<code>{html}</code>) served from disk with its own assets. Edit
      the name above; to change its contents, re-upload an updated <code>.zip</code>. The preview
      renders the live bundle.
    </div>
    <section class="panel preview-only">
      <div class="panel-head"><h2 class="panel-title">Live Preview</h2></div>
      <div class="panel-body preview-body">
        <iframe
          class="frame"
          src={portals.previewURL(id)}
          title="Preview"
          sandbox="allow-scripts allow-forms"
        ></iframe>
      </div>
    </section>
  {:else}
    {#if detected.length > 0}
      <div class="captures">
        <span class="field-label">Fields this portal captures on submit</span>
        <div class="captures-list">
          {#each detected as f}
            <span class="badge badge-warning mono">{f}</span>
          {/each}
        </div>
      </div>
    {/if}

    <div class="split editor-split">
      <section class="panel">
        <div class="panel-head">
          <h2 class="panel-title">HTML Source</h2>
        </div>
        <div class="panel-body src-body">
          <textarea class="input src-area" bind:value={html}></textarea>
        </div>
      </section>

      <section class="panel">
        <div class="panel-head">
          <h2 class="panel-title">Live Preview</h2>
          <div class="head-actions">
            <button
              class="btn btn-sm"
              class:btn-primary={device === 'desktop'}
              onclick={() => (device = 'desktop')}>Desktop</button
            >
            <button
              class="btn btn-sm"
              class:btn-primary={device === 'mobile'}
              onclick={() => (device = 'mobile')}>Mobile</button
            >
          </div>
        </div>
        <div class="panel-body preview-body">
          {#if html}
            <iframe
              class="frame"
              class:mobile={device === 'mobile'}
              srcdoc={html}
              title="Preview"
              sandbox="allow-scripts allow-forms"
            ></iframe>
          {:else}
            <div class="empty-state preview-empty"><p>No HTML content</p></div>
          {/if}
        </div>
      </section>
    </div>
  {/if}
</div>

<style>
  .editor {
    display: flex;
    flex-direction: column;
    gap: var(--space-lg);
  }

  .editor-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-lg);
    flex-wrap: wrap;
  }
  .head-id {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
    flex: 1;
  }
  .crumb {
    font-size: var(--font-size-xs);
    color: var(--text-dim);
  }
  .crumb:hover {
    color: var(--accent-hover);
  }
  .name-input {
    font-size: var(--font-size-lg);
    font-weight: 700;
    letter-spacing: -0.01em;
    max-width: 480px;
    background: transparent;
    border-color: transparent;
    padding-left: var(--space-sm);
  }
  .name-input:hover {
    border-color: var(--border-primary);
  }
  .name-input:focus {
    background: var(--bg-input);
  }

  .head-actions {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
    flex-wrap: wrap;
  }
  .auth-select {
    max-width: 200px;
  }

  .bundle-note {
    background: rgba(245, 158, 11, 0.06);
    border: 1px solid rgba(245, 158, 11, 0.3);
    border-radius: var(--radius-md);
    padding: var(--space-md) var(--space-lg);
    font-size: var(--font-size-sm);
    color: var(--status-warning);
    line-height: 1.6;
  }
  .bundle-note code {
    color: var(--text-primary);
  }

  .captures {
    display: flex;
    flex-direction: column;
    gap: var(--space-sm);
  }
  .captures-list {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-xs);
  }

  .editor-split {
    align-items: stretch;
  }
  .editor-split .panel {
    display: flex;
    flex-direction: column;
    min-height: 600px;
  }
  .editor-split .panel-body {
    flex: 1;
    padding: 0;
    display: flex;
    min-height: 0;
  }

  .src-body {
    background: var(--bg-input);
  }
  .src-area {
    flex: 1;
    width: 100%;
    border: none;
    border-radius: 0;
    resize: vertical;
    min-height: 100%;
    background: transparent;
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    line-height: 1.55;
  }
  .src-area:focus {
    box-shadow: none;
  }

  .preview-body {
    background: #fff;
  }
  .preview-only .preview-body {
    height: 600px;
  }
  .frame {
    width: 100%;
    height: 100%;
    border: none;
    background: #fff;
    display: block;
  }
  .frame.mobile {
    width: 390px;
    max-width: 100%;
    margin: 0 auto;
    border-left: 1px solid var(--border-primary);
    border-right: 1px solid var(--border-primary);
  }
  .preview-empty {
    width: 100%;
    align-self: center;
    color: var(--text-muted);
  }

  @media (max-width: 1000px) {
    .editor-split .panel {
      min-height: 460px;
    }
  }
</style>
