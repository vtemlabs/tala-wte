<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, paid training, paid CTF,
  or any for-profit use requires a license from VTEM Labs. See the LICENSE file.
-->
<script lang="ts">
  // Shows the full license, fetched from /api/wte/license.
  let { open = $bindable(false) }: { open?: boolean } = $props();

  let text = $state('');
  let loading = $state(false);

  $effect(() => {
    if (open && !text && !loading) load();
  });

  async function load() {
    loading = true;
    try {
      const res = await fetch('/api/wte/license');
      text = res.ok ? await res.text() : 'Unable to load the license. See the LICENSE file in the distribution.';
    } catch {
      text = 'Unable to load the license. See the LICENSE file in the distribution.';
    }
    loading = false;
  }
</script>

<svelte:window onkeydown={(e) => { if (e.key === 'Escape' && open) open = false; }} />

{#if open}
  <div class="lic-overlay" onclick={() => open = false} onkeydown={(e) => { if (e.key === 'Escape') open = false; }} role="presentation">
    <div class="lic-modal" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-label="Tala WTE License" tabindex="-1">
      <div class="lic-head">
        <span class="lic-title">Tala WTE License</span>
        <button class="lic-close" onclick={() => open = false} aria-label="Close">×</button>
      </div>
      <div class="lic-body">
        {#if loading}
          <p class="lic-loading">Loading license...</p>
        {:else}
          <pre class="lic-text">{text}</pre>
        {/if}
      </div>
      <div class="lic-foot">
        <button class="btn btn-primary" onclick={() => open = false}>Close</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .lic-overlay { position: fixed; inset: 0; z-index: 600; background: rgba(0,0,0,0.6); display: flex; align-items: center; justify-content: center; padding: var(--space-lg); }
  .lic-modal { width: 100%; max-width: 760px; max-height: 86vh; display: flex; flex-direction: column; background: var(--bg-card); border: 1px solid var(--border-primary); border-radius: var(--radius-lg); box-shadow: var(--shadow-lg); overflow: hidden; }
  .lic-head { display: flex; align-items: center; justify-content: space-between; padding: var(--space-md) var(--space-xl); border-bottom: 1px solid var(--border-primary); }
  .lic-title { font-size: var(--font-size-md); font-weight: 700; color: var(--text-primary); }
  .lic-close { background: none; border: none; color: var(--text-muted); font-size: 22px; line-height: 1; cursor: pointer; padding: 0 var(--space-xs); }
  .lic-close:hover { color: var(--text-primary); }
  .lic-body { overflow-y: auto; padding: var(--space-xl); background: var(--bg-primary); }
  .lic-text { margin: 0; font-family: var(--font-mono); font-size: var(--font-size-xs); line-height: 1.6; color: var(--text-secondary); white-space: pre-wrap; word-break: break-word; }
  .lic-loading { color: var(--text-muted); font-size: var(--font-size-sm); }
  .lic-foot { display: flex; align-items: center; justify-content: flex-end; gap: var(--space-lg); padding: var(--space-md) var(--space-xl); border-top: 1px solid var(--border-primary); }
</style>
