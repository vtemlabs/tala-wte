<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government
  use require a license from VTEM Labs. The Software may not be copied or
  redistributed. See the LICENSE file.
-->
<script lang="ts">
  import type { AdapterSwap } from '$lib/api';

  let {
    open = $bindable(false),
    swap,
    busy = false,
    onConfirm,
    onCancel
  }: {
    open?: boolean;
    swap: AdapterSwap | null;
    busy?: boolean;
    onConfirm: () => void;
    onCancel: () => void;
  } = $props();

  function cancel() {
    open = false;
    onCancel();
  }
</script>

{#if open && swap}
  <button class="backdrop" onclick={cancel} aria-label="Close" tabindex="-1"></button>
  <div class="modal" role="dialog" aria-modal="true" aria-labelledby="swap-title">
    <div class="modal-header">
      <h2 id="swap-title">Wireless adapter changed</h2>
      <button class="close-btn" onclick={cancel} disabled={busy}>×</button>
    </div>

    <div class="modal-body">
      <p class="line">
        The adapter this network was saved with, <span class="mono">{swap.missing}</span>, is not
        connected.
      </p>
      <p class="line">
        Available adapter: <strong>{swap.proposed.label}</strong>
        <span class="mono">({swap.proposed.interface})</span>. Use it for this network?
      </p>

      {#if !swap.band_ok}
        <div class="warn">{swap.band_reason}.</div>
      {/if}
    </div>

    <div class="modal-footer">
      <button class="btn btn-secondary" onclick={cancel} disabled={busy}>Cancel</button>
      <button class="btn btn-primary" onclick={onConfirm} disabled={busy}>
        {busy ? 'Starting…' : `Use ${swap.proposed.interface}`}
      </button>
    </div>
  </div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.65);
    z-index: 500;
    border: none;
    cursor: pointer;
  }
  .modal {
    position: fixed;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    background: var(--bg-secondary);
    border: 1px solid var(--border-primary);
    border-radius: var(--radius-md);
    width: min(520px, 92vw);
    z-index: 510;
    box-shadow: 0 18px 60px rgba(0, 0, 0, 0.5);
  }
  .modal-header {
    padding: var(--space-lg) var(--space-xl);
    border-bottom: 1px solid var(--border-primary);
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .modal-header h2 {
    font-size: var(--font-size-lg);
    margin: 0;
    color: var(--text-primary);
  }
  .close-btn {
    background: none;
    border: none;
    font-size: var(--font-size-xl);
    line-height: 1;
    color: var(--text-dim);
    cursor: pointer;
    padding: 0 var(--space-sm);
  }
  .close-btn:hover:not(:disabled) {
    color: var(--text-primary);
  }
  .close-btn:disabled {
    opacity: 0.5;
    cursor: default;
  }
  .modal-body {
    padding: var(--space-lg) var(--space-xl);
  }
  .line {
    color: var(--text-secondary);
    line-height: 1.6;
    margin: 0 0 var(--space-md);
  }
  .line:last-child {
    margin-bottom: 0;
  }
  .mono {
    font-family: var(--font-mono);
    color: var(--text-primary);
  }
  .warn {
    margin-top: var(--space-md);
    padding: var(--space-sm) var(--space-md);
    border: 1px solid var(--color-yellow);
    border-radius: var(--radius-sm);
    color: var(--color-yellow);
    font-size: var(--font-size-sm);
    line-height: 1.5;
  }
  .modal-footer {
    padding: var(--space-md) var(--space-xl) var(--space-lg);
    display: flex;
    justify-content: flex-end;
    gap: var(--space-md);
  }
</style>
