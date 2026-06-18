<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government
  use require a license from VTEM Labs. The Software may not be copied or
  redistributed. See the LICENSE file.
-->
<script lang="ts">
  import { toast } from '$lib/stores/toast';
  import { fly } from 'svelte/transition';
</script>

<div class="toast-container">
  {#each $toast as t (t.id)}
    <div
      class="toast {t.type}"
      in:fly={{ y: -20, duration: 200 }}
      out:fly={{ y: -20, duration: 200 }}
    >
      {#if t.type === 'err'}
        <svg viewBox="0 0 24 24" class="icon"
          ><path
            fill="currentColor"
            d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z"
          /></svg
        >
      {:else if t.type === 'success'}
        <svg viewBox="0 0 24 24" class="icon"
          ><path fill="currentColor" d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z" /></svg
        >
      {:else if t.type === 'warning'}
        <svg viewBox="0 0 24 24" class="icon"
          ><path fill="currentColor" d="M1 21h22L12 2 1 21zm12-3h-2v-2h2v2zm0-4h-2v-4h2v4z" /></svg
        >
      {:else}
        <svg viewBox="0 0 24 24" class="icon"
          ><path
            fill="currentColor"
            d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z"
          /></svg
        >
      {/if}
      <div class="content">
        {t.message}
      </div>
      <button class="close-btn" aria-label="Close Toast" onclick={() => toast.remove(t.id)}>
        <svg viewBox="0 0 24 24"
          ><path
            fill="currentColor"
            d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12 19 6.41z"
          /></svg
        >
      </button>
    </div>
  {/each}
</div>

<style>
  .toast-container {
    position: fixed;
    top: 1rem;
    right: 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    z-index: 10000;
    pointer-events: none;
  }

  .toast {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.75rem 1rem;
    background: #111111;
    border: 1px solid #333333;
    border-radius: 4px;
    color: #ffffff;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.5);
    pointer-events: auto;
    min-width: 300px;
    max-width: 450px;
    font-family: var(--font-mono, monospace);
    font-size: 0.85rem;
  }

  .toast.err {
    border-left: 3px solid #ff4444;
  }

  .toast.success {
    border-left: 3px solid #00cc66;
  }

  .toast.warning {
    border-left: 3px solid #ffaa00;
  }

  .toast.info {
    border-left: 3px solid #0088ff;
  }

  .icon {
    width: 1.25rem;
    height: 1.25rem;
    flex-shrink: 0;
  }

  .toast.err .icon {
    color: #ff4444;
  }
  .toast.success .icon {
    color: #00cc66;
  }
  .toast.warning .icon {
    color: #ffaa00;
  }
  .toast.info .icon {
    color: #0088ff;
  }

  .content {
    flex: 1;
    word-break: break-word;
    line-height: 1.4;
  }

  .close-btn {
    background: none;
    border: none;
    color: #666666;
    cursor: pointer;
    padding: 0.25rem;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: color 0.2s;
  }

  .close-btn:hover {
    color: #ffffff;
  }

  .close-btn svg {
    width: 1rem;
    height: 1rem;
  }
</style>
