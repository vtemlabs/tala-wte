<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government
  use require a license from VTEM Labs. The Software may not be copied or
  redistributed. See the LICENSE file.
-->
<script lang="ts">
  // Floating, draggable, resizable live-log window mirroring the Terminal modal chrome.
  let {
    open = $bindable(false),
    title = 'Live Log',
    streaming = false,
    lines = []
  }: { open?: boolean; title?: string; streaming?: boolean; lines?: string[] } = $props();

  let minimized = $state(false);
  let maximized = $state(false);
  let x = $state(0),
    y = $state(0),
    w = $state(860),
    h = $state(560);
  let started = false;
  let bodyEl: HTMLDivElement | undefined = $state();

  // Follow the tail when new lines arrive and the viewer is already near bottom.
  $effect(() => {
    const _ = lines.length; // reactive dependency on line count
    if (!bodyEl || minimized) return;
    const atBottom = bodyEl.scrollTop + bodyEl.clientHeight >= bodyEl.scrollHeight - 60;
    if (atBottom)
      requestAnimationFrame(() => {
        if (bodyEl) bodyEl.scrollTop = bodyEl.scrollHeight;
      });
  });

  $effect(() => {
    if (open && !started) {
      started = true;
      x = Math.max(20, (window.innerWidth - w) / 2);
      y = Math.max(20, (window.innerHeight - h) / 4);
      requestAnimationFrame(() => {
        if (bodyEl) bodyEl.scrollTop = bodyEl.scrollHeight;
      });
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

  // Drag (title bar) + resize (edge/corner handles), same model as Terminal.
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
        h = Math.max(220, Math.min(sH + e.clientY - sy, window.innerHeight - y - 8));
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
    class="tw"
    class:max={maximized}
    class:min={minimized}
    style={maximized || minimized ? '' : `left:${x}px;top:${y}px;width:${w}px;height:${h}px;`}
    role="dialog"
    aria-label={title}
  >
    <div
      class="tw-title"
      onmousedown={(e) => start(e, 'drag')}
      onclick={(e) => {
        if (minimized && !(e.target as HTMLElement).closest('button')) restore();
      }}
      role="toolbar"
      tabindex="-1"
    >
      <span class="tw-name">
        <span class="tw-led" class:on={streaming}></span>
        {title}{#if minimized}
          (click to restore){/if}
      </span>
      <div class="tw-win">
        <button onclick={toggleMin} title="Minimize" aria-label="Minimize">-</button>
        <button onclick={toggleMax} title={maximized ? 'Restore' : 'Maximize'} aria-label="Maximize"
          >▢</button
        >
        <button onclick={close} title="Close" aria-label="Close">×</button>
      </div>
    </div>

    <div class="tw-body" class:hidden={minimized} bind:this={bodyEl}>
      {#if lines.length === 0}
        <div class="logempty">No activity yet - start the network and connect a client.</div>
      {:else}
        {#each lines as line}<div class="logline">{line}</div>{/each}
      {/if}
    </div>

    {#if !maximized && !minimized}
      <div
        class="rz rz-e"
        onmousedown={(e) => start(e, 'e')}
        role="separator"
        tabindex="-1"
        aria-label="Resize width"
      ></div>
      <div
        class="rz rz-s"
        onmousedown={(e) => start(e, 's')}
        role="separator"
        tabindex="-1"
        aria-label="Resize height"
      ></div>
      <div
        class="rz rz-se"
        onmousedown={(e) => start(e, 'se')}
        role="separator"
        tabindex="-1"
        aria-label="Resize"
      ></div>
    {/if}
  </div>
{/if}

<style>
  .tw {
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
  .tw.max {
    left: 2vw !important;
    top: 2vh !important;
    width: 96vw !important;
    height: 92vh !important;
  }
  .tw.min {
    left: auto !important;
    right: 16px !important;
    top: auto !important;
    bottom: 0 !important;
    width: 320px !important;
    height: auto !important;
    border-bottom-left-radius: 0 !important;
    border-bottom-right-radius: 0 !important;
  }

  .tw-title {
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
  .tw-name {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    color: var(--text-muted);
    font-family: var(--font-mono, monospace);
  }
  .tw-led {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--text-dim);
  }
  .tw-led.on {
    background: var(--color-green);
    box-shadow: 0 0 8px var(--color-green);
  }
  .tw-win {
    display: flex;
    gap: 2px;
  }
  .tw-win button {
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
  .tw-win button:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .tw-body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    background: var(--bg-primary);
    padding: var(--space-md) var(--space-lg);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    line-height: 1.55;
    color: #c9d1d9;
    white-space: pre-wrap;
    word-break: break-word;
  }
  .tw-body.hidden {
    display: none;
  }
  .logline {
    color: #c9d1d9;
  }
  .logline:hover {
    color: var(--text-primary);
  }
  .logempty {
    color: var(--text-dim);
    font-style: italic;
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

  :global(.tw .tw-body::-webkit-scrollbar) {
    width: 8px;
  }
  :global(.tw .tw-body::-webkit-scrollbar-track) {
    background: transparent;
  }
  :global(.tw .tw-body::-webkit-scrollbar-thumb) {
    background: var(--border-primary);
    border-radius: 4px;
  }
  :global(.tw .tw-body::-webkit-scrollbar-thumb:hover) {
    background: var(--text-dim);
  }
</style>
