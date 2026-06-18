<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government
  use require a license from VTEM Labs. The Software may not be copied or
  redistributed. See the LICENSE file.
-->
<script lang="ts">
  let {
    open = $bindable(false),
    portalId,
    portalName = ''
  }: {
    open?: boolean;
    portalId: string;
    portalName?: string;
  } = $props();

  let device = $state<'desktop' | 'mobile'>('desktop');

  function close() {
    open = false;
  }
</script>

<svelte:window
  onkeydown={(e) => {
    if (e.key === 'Escape' && open) close();
  }}
/>

{#if open && portalId}
  <button class="backdrop" onclick={close} aria-label="Close" tabindex="-1"></button>
  <div class="modal" role="dialog" aria-modal="true" aria-label="Portal preview">
    <div class="modal-header">
      <span class="title">Portal preview{portalName ? ` - ${portalName}` : ''}</span>
      <div class="seg">
        <button
          class="seg-btn"
          class:active={device === 'desktop'}
          onclick={() => (device = 'desktop')}>Desktop</button
        >
        <button
          class="seg-btn"
          class:active={device === 'mobile'}
          onclick={() => (device = 'mobile')}>Mobile</button
        >
      </div>
      <button class="close-btn" onclick={close} aria-label="Close">×</button>
    </div>

    <div class="stage" class:mobile={device === 'mobile'}>
      <div class="viewport" class:phone={device === 'mobile'}>
        <iframe
          title="Portal preview"
          src="/api/wte/portals/{portalId}/preview"
          sandbox="allow-same-origin"
        ></iframe>
      </div>
    </div>
  </div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.7);
    z-index: 600;
    border: none;
    cursor: pointer;
  }
  .modal {
    position: fixed;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    width: min(1100px, 94vw);
    height: min(860px, 92vh);
    background: var(--bg-secondary);
    border: 1px solid var(--border-primary);
    border-radius: var(--radius-md);
    z-index: 610;
    display: flex;
    flex-direction: column;
    box-shadow: 0 18px 60px rgba(0, 0, 0, 0.6);
  }
  .modal-header {
    display: flex;
    align-items: center;
    gap: var(--space-md);
    padding: var(--space-md) var(--space-lg);
    border-bottom: 1px solid var(--border-primary);
  }
  .title {
    font-size: var(--font-size-md);
    color: var(--text-primary);
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .seg {
    display: flex;
    border: 1px solid var(--border-primary);
    border-radius: var(--radius-sm);
    overflow: hidden;
  }
  .seg-btn {
    background: var(--bg-input);
    border: none;
    color: var(--text-secondary);
    padding: 5px 14px;
    font-size: var(--font-size-sm);
    cursor: pointer;
  }
  .seg-btn.active {
    background: var(--color-cyan);
    color: var(--bg-primary);
  }
  .close-btn {
    background: none;
    border: none;
    font-size: 1.5rem;
    line-height: 1;
    color: var(--text-dim);
    cursor: pointer;
    padding: 0 var(--space-sm);
  }
  .close-btn:hover {
    color: var(--text-primary);
  }
  .stage {
    flex: 1;
    min-height: 0;
    padding: var(--space-lg);
    display: flex;
    overflow: auto;
  }
  .stage.mobile {
    align-items: flex-start;
    justify-content: center;
  }
  .viewport {
    flex: 1;
    min-height: 0;
    border: 1px solid var(--border-primary);
    border-radius: var(--radius-sm);
    overflow: hidden;
    background: #fff;
  }
  .viewport.phone {
    flex: none;
    width: 390px;
    height: 100%;
    max-height: 844px;
    border-radius: 22px;
    border: 6px solid #111;
  }
  iframe {
    width: 100%;
    height: 100%;
    border: 0;
    display: block;
    background: #fff;
  }
</style>
