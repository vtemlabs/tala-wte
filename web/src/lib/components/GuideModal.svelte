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
  import { lightbox } from '$lib/lightbox';
  import { docScale, zoomDoc } from '$lib/stores/docScale';

  let {
    open = $bindable(false),
    title = 'Guide',
    doc = ''
  }: { open?: boolean; title?: string; doc?: string } = $props();

  let minimized = $state(false);
  let maximized = $state(false);
  let x = $state(0),
    y = $state(0),
    w = $state(1040),
    h = $state(600);
  let started = false;

  const html = $derived(mdToHtml(doc));

  $effect(() => {
    if (open && !started) {
      started = true;
      // Open at a proper 16:9, sized to the viewport.
      w = Math.min(1120, window.innerWidth - 48);
      h = Math.min(Math.round((w * 9) / 16), window.innerHeight - 48);
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
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <div
      class="gw-title"
      onmousedown={(e) => start(e, 'drag')}
      onclick={(e) => {
        if (minimized && !(e.target as HTMLElement).closest('button')) restore();
      }}
      role="toolbar"
      tabindex="-1"
    >
      <span class="gw-name"
        >{title}{#if minimized}
          (click to restore){/if}</span
      >
      <div class="gw-font">
        <button onclick={() => zoomDoc(-0.1)} title="Smaller text" aria-label="Smaller text"
          >A-</button
        >
        <span class="gw-fs">{Math.round($docScale * 100)}%</span>
        <button onclick={() => zoomDoc(0.1)} title="Larger text" aria-label="Larger text">A+</button
        >
      </div>
      <div class="gw-win">
        <button onclick={toggleMin} title="Minimize" aria-label="Minimize">-</button>
        <button onclick={toggleMax} title={maximized ? 'Restore' : 'Maximize'} aria-label="Maximize"
          >▢</button
        >
        <button onclick={close} title="Close" aria-label="Close">×</button>
      </div>
    </div>

    <div class="gw-body" class:hidden={minimized}>
      <div
        class="tala-doc"
        use:lightbox
        style="font-size: calc(var(--font-size-sm) * 1.1 * {$docScale})"
      >
        {@html html}
        <footer class="gw-doc-footer">
          <img src="/brand/vtem-labs.png" alt="VTEM Labs" />
          <span>(c) 2026 VTEM Labs, Inc. All rights reserved.</span>
          <span class="sep">|</span>
          <a href="https://vtemlabs.com" target="_blank" rel="noopener noreferrer">vtemlabs.com</a>
        </footer>
      </div>
    </div>

    {#if !maximized && !minimized}
      <div
        class="rz rz-e"
        onmousedown={(e) => start(e, 'e')}
        role="button"
        tabindex="-1"
        aria-label="Resize width"
      ></div>
      <div
        class="rz rz-s"
        onmousedown={(e) => start(e, 's')}
        role="button"
        tabindex="-1"
        aria-label="Resize height"
      ></div>
      <div
        class="rz rz-se"
        onmousedown={(e) => start(e, 'se')}
        role="button"
        tabindex="-1"
        aria-label="Resize"
      ></div>
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
  /* Mobile: a draggable/resizable floating window is useless on a phone, so the
     default (non-min) state fills the screen. */
  @media (max-width: 768px) {
    .gw:not(.min) {
      left: 0 !important;
      top: 0 !important;
      width: 100vw !important;
      height: 100dvh !important;
      border-radius: 0 !important;
      border-width: 0 !important;
    }
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
    font-size: var(--font-size-xs);
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
    font-size: var(--font-size-base);
    line-height: 1;
    border-radius: 4px;
  }
  .gw-win button:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }
  .gw-font {
    display: flex;
    align-items: center;
    gap: 2px;
    margin-right: var(--space-sm);
  }
  .gw-font button {
    min-width: 26px;
    height: 22px;
    padding: 0 5px;
    background: none;
    border: none;
    cursor: pointer;
    color: var(--text-muted);
    font-size: var(--font-size-xs);
    font-weight: 600;
    border-radius: 4px;
  }
  .gw-font button:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }
  .gw-fs {
    font-size: var(--font-size-2xs);
    color: var(--text-dim);
    min-width: 36px;
    text-align: center;
    font-variant-numeric: tabular-nums;
  }
  :global(.gw .gw-doc-footer) {
    display: flex;
    align-items: center;
    justify-content: center;
    flex-wrap: wrap;
    gap: var(--space-sm);
    margin-top: var(--space-2xl);
    padding-top: var(--space-lg);
    border-top: 1px solid var(--border-primary);
    font-size: var(--font-size-xs);
    color: var(--text-dim);
  }
  :global(.gw .gw-doc-footer img) {
    height: 14px;
    max-height: 14px;
    width: auto;
    margin: 0;
    border: none;
    border-radius: 0;
    background: none;
    opacity: 0.85;
    cursor: default;
  }
  :global(.gw .gw-doc-footer .sep) {
    color: var(--border-secondary);
  }
  :global(.gw .gw-doc-footer a) {
    color: var(--text-muted);
    text-decoration: none;
  }
  :global(.gw .gw-doc-footer a:hover) {
    color: var(--accent-hover);
    text-decoration: underline;
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
  /* Doc body styling is global (.tala-doc in app.css), shared with the full-page guides. */
  :global(.gw .gw-body::-webkit-scrollbar) {
    width: 8px;
  }
  :global(.gw .gw-body::-webkit-scrollbar-thumb) {
    background: var(--border-primary);
    border-radius: 4px;
  }
</style>
