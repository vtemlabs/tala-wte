<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government use
  require a license from VTEM Labs. See the LICENSE file.
-->
<script lang="ts">
  // Floating, draggable, resizable guide window so a guide can be referenced
  // side by side while adjusting settings. Same chrome as the Live Log window.
  import { mdToHtml } from '$lib/md';

  let {
    open = $bindable(false),
    title = 'Guide',
    doc = ''
  }: { open?: boolean; title?: string; doc?: string } = $props();

  let minimized = $state(false);
  let maximized = $state(false);
  let x = $state(0),
    y = $state(0),
    w = $state(720),
    h = $state(620);
  let started = false;

  const html = $derived(mdToHtml(doc));

  $effect(() => {
    if (open && !started) {
      started = true;
      x = Math.max(20, window.innerWidth - w - 40);
      y = Math.max(20, (window.innerHeight - h) / 4);
    }
    if (!open) {
      started = false;
      minimized = false;
      maximized = false;
    }
  });

  function close() {
    open = false;
  }
  function toggleMin() {
    minimized = !minimized;
  }
  function toggleMax() {
    if (minimized) {
      minimized = false;
      return;
    }
    maximized = !maximized;
  }
  function restore() {
    if (minimized) minimized = false;
  }

  let mode: '' | 'drag' | 'e' | 's' | 'se' = '';
  let sx = 0,
    sy = 0,
    sX = 0,
    sY = 0,
    sW = 0,
    sH = 0;
  function onMove(e: MouseEvent) {
    if (mode === 'drag') {
      x = Math.min(Math.max(0, sX + e.clientX - sx), window.innerWidth - 120);
      y = Math.min(Math.max(0, sY + e.clientY - sy), window.innerHeight - 36);
    } else if (mode) {
      if (mode === 'e' || mode === 'se')
        w = Math.max(360, Math.min(sW + e.clientX - sx, window.innerWidth - x - 8));
      if (mode === 's' || mode === 'se')
        h = Math.max(240, Math.min(sH + e.clientY - sy, window.innerHeight - y - 8));
    }
  }
  function onUp() {
    mode = '';
    window.removeEventListener('mousemove', onMove);
    window.removeEventListener('mouseup', onUp);
  }
  function start(e: MouseEvent, m: typeof mode) {
    if (maximized || minimized) return;
    mode = m;
    sx = e.clientX;
    sy = e.clientY;
    sX = x;
    sY = y;
    sW = w;
    sH = h;
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
    e.preventDefault();
  }
</script>

{#if open}
  <div
    class="gw"
    class:max={maximized}
    class:min={minimized}
    style={maximized || minimized ? '' : `left:${x}px;top:${y}px;width:${w}px;height:${h}px;`}
    role="dialog"
    aria-label={title}
  >
    <div
      class="gw-title"
      onmousedown={(e) => start(e, 'drag')}
      onclick={(e) => {
        if (minimized && !(e.target as HTMLElement).closest('button')) restore();
      }}
      role="toolbar"
      tabindex="-1"
    >
      <span class="gw-name">{title}{#if minimized} (click to restore){/if}</span>
      <div class="gw-win">
        <button onclick={toggleMin} title="Minimize" aria-label="Minimize">-</button>
        <button onclick={toggleMax} title={maximized ? 'Restore' : 'Maximize'} aria-label="Maximize">▢</button>
        <button onclick={close} title="Close" aria-label="Close">×</button>
      </div>
    </div>

    <div class="gw-body" class:hidden={minimized}>
      <div class="guide-prose">{@html html}</div>
    </div>

    {#if !maximized && !minimized}
      <div class="rz rz-e" onmousedown={(e) => start(e, 'e')} role="separator" tabindex="-1" aria-label="Resize width"></div>
      <div class="rz rz-s" onmousedown={(e) => start(e, 's')} role="separator" tabindex="-1" aria-label="Resize height"></div>
      <div class="rz rz-se" onmousedown={(e) => start(e, 'se')} role="separator" tabindex="-1" aria-label="Resize"></div>
    {/if}
  </div>
{/if}

<style>
  .gw {
    position: fixed;
    z-index: 500;
    background: var(--bg-primary);
    border: 1px solid var(--border-primary);
    border-radius: 8px;
    box-shadow: 0 12px 48px rgba(0, 0, 0, 0.65);
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .gw.max {
    left: 2vw !important;
    top: 2vh !important;
    width: 96vw !important;
    height: 92vh !important;
  }
  .gw.min {
    left: auto !important;
    right: 16px !important;
    top: auto !important;
    bottom: 0 !important;
    width: 320px !important;
    height: auto !important;
    border-bottom-left-radius: 0 !important;
    border-bottom-right-radius: 0 !important;
  }
  .gw-title {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 6px 10px;
    background: var(--bg-secondary);
    border-bottom: 1px solid var(--border-primary);
    cursor: move;
    user-select: none;
    flex-shrink: 0;
  }
  .gw-name {
    font-size: 12px;
    color: var(--text-muted);
    letter-spacing: 0.04em;
    text-transform: uppercase;
  }
  .gw-win {
    display: flex;
    gap: 2px;
  }
  .gw-win button {
    width: 28px;
    height: 22px;
    background: none;
    border: none;
    cursor: pointer;
    color: var(--text-muted);
    font-size: 14px;
    line-height: 1;
    border-radius: 4px;
  }
  .gw-win button:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }
  .gw-body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    background: var(--bg-primary);
    padding: var(--space-lg) var(--space-xl);
  }
  .gw-body.hidden {
    display: none;
  }
  .rz {
    position: absolute;
    z-index: 2;
  }
  .rz-e {
    top: 0;
    right: 0;
    width: 6px;
    height: 100%;
    cursor: ew-resize;
  }
  .rz-s {
    left: 0;
    bottom: 0;
    height: 6px;
    width: 100%;
    cursor: ns-resize;
  }
  .rz-se {
    right: 0;
    bottom: 0;
    width: 14px;
    height: 14px;
    cursor: nwse-resize;
  }
  :global(.gw .guide-prose h2) {
    font-size: var(--font-size-lg);
    color: var(--text-primary);
    margin: var(--space-lg) 0 var(--space-sm);
  }
  :global(.gw .guide-prose h3) {
    font-size: var(--font-size-md);
    color: var(--text-primary);
    margin: var(--space-md) 0 var(--space-xs);
  }
  :global(.gw .guide-prose p),
  :global(.gw .guide-prose li) {
    color: var(--text-secondary);
    font-size: var(--font-size-sm);
    line-height: 1.6;
  }
  :global(.gw .guide-prose code) {
    font-family: var(--font-mono);
    background: var(--bg-input);
    padding: 1px 5px;
    border-radius: 4px;
    font-size: 0.85em;
    color: var(--accent-hover);
  }
  :global(.gw .guide-prose strong) {
    color: var(--text-primary);
  }
  :global(.gw .gw-body::-webkit-scrollbar) {
    width: 8px;
  }
  :global(.gw .gw-body::-webkit-scrollbar-thumb) {
    background: var(--border-primary);
    border-radius: 4px;
  }
</style>
