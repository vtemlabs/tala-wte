<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government
  use require a license from VTEM Labs. The Software may not be copied or
  redistributed. See the LICENSE file.
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { Terminal } from '@xterm/xterm';
  import { FitAddon } from '@xterm/addon-fit';
  import { WebLinksAddon } from '@xterm/addon-web-links';
  import '@xterm/xterm/css/xterm.css';
  import { pb } from '$lib/api';

  let { open = $bindable(false) }: { open?: boolean } = $props();

  // Lightweight reactive tab descriptors; heavy xterm/WebSocket objects live in a
  // plain Map so Svelte's deep proxy never wraps them.
  type Tab = { id: string; label: string; connected: boolean };
  type Sess = { term: Terminal; fit: FitAddon; ws: WebSocket | null; el: HTMLDivElement };

  let tabs = $state<Tab[]>([]);
  let activeId = $state('');
  let fontSize = $state(13);
  let editingId = $state<string | null>(null);
  let editLabel = $state('');

  let minimized = $state(false);
  let maximized = $state(false);
  let x = $state(0),
    y = $state(0),
    w = $state(840),
    h = $state(520);

  let containerEl: HTMLDivElement | undefined = $state();
  const sess = new Map<string, Sess>();
  let counter = 0;
  let started = false;

  function setConnected(id: string, v: boolean) {
    const t = tabs.find((t) => t.id === id);
    if (t) t.connected = v;
  }

  // FitAddon over-counts columns (14px scrollbar gutter), so after fit() recompute
  // columns from the actual rendered cell width with a ~2px gutter.
  function refit(s: Sess) {
    try {
      s.fit.fit();
    } catch {
      return;
    }
    try {
      const screen = s.el.querySelector('.xterm-screen') as HTMLElement | null;
      const parentW = s.el.clientWidth;
      if (!screen || !s.term.cols || !parentW) return;
      const cellW = screen.getBoundingClientRect().width / s.term.cols;
      if (!cellW || !isFinite(cellW) || cellW <= 0) return;
      const cols = Math.max(2, Math.floor((parentW - 2) / cellW));
      if (cols !== s.term.cols) s.term.resize(cols, s.term.rows);
    } catch {
      /* not ready */
    }
  }

  function fitActive() {
    if (minimized) return;
    const s = sess.get(activeId);
    if (s) refit(s);
  }

  function connectWS(id: string, s: Sess) {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const token = encodeURIComponent(pb.authStore.token);
    const ws = new WebSocket(`${proto}//${location.host}/api/wte/terminal/ws?token=${token}`);
    ws.binaryType = 'arraybuffer';
    s.ws = ws;
    ws.onopen = () => {
      setConnected(id, true);
      ws.send(JSON.stringify({ type: 'resize', cols: s.term.cols, rows: s.term.rows }));
    };
    ws.onclose = () => {
      setConnected(id, false);
      s.term.write('\r\n\x1b[31m[session closed]\x1b[0m\r\n');
    };
    ws.onerror = () => setConnected(id, false);
    ws.onmessage = (ev) => {
      if (typeof ev.data === 'string') s.term.write(ev.data);
      else s.term.write(new Uint8Array(ev.data));
    };
    s.term.onData((d) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(d);
    });
    s.term.onResize(({ cols, rows }) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'resize', cols, rows }));
    });
  }

  // xterm renders to canvas and cannot take CSS vars, so resolve them once here.
  function cssVar(name: string, fallback: string): string {
    if (typeof document === 'undefined') return fallback;
    return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback;
  }

  // Map the app theme palette onto an xterm theme + ANSI colour table.
  function xtermTheme() {
    return {
      background: cssVar('--bg-primary', '#0a0a0a'),
      foreground: cssVar('--text-primary', '#e5e5e5'),
      cursor: cssVar('--color-green', '#22c55e'),
      cursorAccent: cssVar('--bg-primary', '#0a0a0a'),
      selectionBackground: cssVar('--border-primary', '#262626'),
      black: cssVar('--bg-tertiary', '#171717'),
      red: cssVar('--color-red', '#ef4444'),
      green: cssVar('--color-green', '#22c55e'),
      yellow: cssVar('--color-yellow', '#eab308'),
      blue: cssVar('--color-blue', '#3b82f6'),
      magenta: cssVar('--color-purple', '#8b5cf6'),
      cyan: cssVar('--color-cyan', '#22d3ee'),
      white: cssVar('--text-secondary', '#b5b5b5'),
      brightBlack: cssVar('--text-dim', '#737373'),
      brightRed: cssVar('--color-red', '#ef4444'),
      brightGreen: cssVar('--color-green-light', '#4ade80'),
      brightYellow: cssVar('--color-yellow', '#eab308'),
      brightBlue: cssVar('--color-blue-light', '#93c5fd'),
      brightMagenta: cssVar('--color-purple-light', '#c4b5fd'),
      brightCyan: cssVar('--color-cyan', '#22d3ee'),
      brightWhite: cssVar('--text-primary', '#e5e5e5')
    };
  }

  async function addTab() {
    if (!containerEl) return;
    counter++;
    const id = 'term-' + counter;
    const term = new Terminal({
      cursorBlink: true,
      fontSize,
      // Unique family name pins xterm to the bundled Nerd Font so glyphs do not
      // clip and it never falls back to a locally installed Nerd Font.
      fontFamily: "'WTE Mono', ui-monospace, Menlo, monospace",
      scrollback: 5000,
      theme: xtermTheme()
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    term.loadAddon(new WebLinksAddon());
    const el = document.createElement('div');
    // ~10px left inset matches the thin scrollbar gutter on the right.
    el.style.cssText = 'position:absolute;top:3px;left:10px;right:0;bottom:2px;display:none;';
    containerEl.appendChild(el);

    const s: Sess = { term, fit, ws: null, el };
    sess.set(id, s);
    tabs.push({ id, label: 'Shell ' + counter, connected: false });

    // Preload both font weights before term.open; xterm measures the cell on open
    // and would otherwise lock to fallback metrics (clipped glyphs, bad $COLUMNS).
    if (typeof document !== 'undefined' && document.fonts) {
      try {
        await Promise.all([
          document.fonts.load("400 16px 'WTE Mono'"),
          document.fonts.load("700 16px 'WTE Mono'")
        ]);
      } catch {
        /* measure with whatever is available */
      }
    }

    // Open on a visible element so the char-size measurement is correct.
    activeId = id;
    for (const [sid, other] of sess) other.el.style.display = sid === id ? 'block' : 'none';
    term.open(el);
    refit(s);
    try {
      term.refresh(0, term.rows - 1);
    } catch {
      /* refresh is best-effort */
    }

    connectWS(id, s);
    setTimeout(() => {
      refit(s);
      s.term.focus();
    }, 30);
  }

  function switchTo(id: string) {
    activeId = id;
    for (const [sid, s] of sess) s.el.style.display = sid === id ? 'block' : 'none';
    setTimeout(() => {
      const s = sess.get(id);
      if (s) {
        refit(s);
        s.term.focus();
      }
    }, 30);
  }

  function closeTab(id: string) {
    const s = sess.get(id);
    if (s) {
      s.ws?.close();
      s.term.dispose();
      s.el.remove();
      sess.delete(id);
    }
    const idx = tabs.findIndex((t) => t.id === id);
    if (idx >= 0) tabs.splice(idx, 1);
    if (activeId === id && tabs.length) switchTo(tabs[Math.min(idx, tabs.length - 1)].id);
    if (!tabs.length) addTab();
  }

  function setFont(delta: number) {
    fontSize = Math.min(28, Math.max(8, fontSize + delta));
    for (const s of sess.values()) {
      s.term.options.fontSize = fontSize;
      refit(s);
    }
  }

  function startRename(t: Tab) {
    editingId = t.id;
    editLabel = t.label;
    setTimeout(() => {
      const i = document.querySelector('.tt-input') as HTMLInputElement | null;
      i?.focus();
      i?.select();
    }, 0);
  }
  function finishRename(t: Tab) {
    if (editLabel.trim()) t.label = editLabel.trim();
    editingId = null;
  }

  function closeAll() {
    for (const s of sess.values()) {
      s.ws?.close();
      s.term.dispose();
      s.el.remove();
    }
    sess.clear();
    tabs = [];
    started = false;
    open = false;
    minimized = false;
    maximized = false;
  }
  function toggleMin() {
    minimized = !minimized;
    if (!minimized) setTimeout(fitActive, 30);
  }
  function restore() {
    if (minimized) {
      minimized = false;
      setTimeout(fitActive, 30);
    }
  }
  function toggleMax() {
    if (minimized) {
      minimized = false;
      setTimeout(fitActive, 30);
      return;
    }
    maximized = !maximized;
    setTimeout(fitActive, 30);
  }

  // Open the first tab once the modal + container are mounted.
  $effect(() => {
    if (open && containerEl && !started) {
      started = true;
      x = Math.max(20, (window.innerWidth - w) / 2);
      y = Math.max(20, (window.innerHeight - h) / 4);
      addTab();
    }
  });

  // Drag (title bar) + resize (edge/corner handles).
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
        w = Math.max(380, Math.min(sW + e.clientX - sx, window.innerWidth - x - 8));
      if (mode === 's' || mode === 'se')
        h = Math.max(240, Math.min(sH + e.clientY - sy, window.innerHeight - y - 8));
    }
  }
  function onUp() {
    const resized = mode === 'e' || mode === 's' || mode === 'se';
    mode = '';
    window.removeEventListener('mousemove', onMove);
    window.removeEventListener('mouseup', onUp);
    // Reflow once after the drag settles; fitting per mousemove floods the shell
    // with SIGWINCH and garbles the prompt.
    if (resized) setTimeout(fitActive, 0);
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

  onMount(() => {
    const r = () => fitActive();
    window.addEventListener('resize', r);
    return () => {
      window.removeEventListener('resize', r);
      closeAll();
    };
  });
</script>

{#if open}
  <div
    class="tw"
    class:max={maximized}
    class:min={minimized}
    style={maximized || minimized ? '' : `left:${x}px;top:${y}px;width:${w}px;height:${h}px;`}
    role="dialog"
    aria-label="Terminal"
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
      <span class="tw-name"
        >Terminal{#if minimized}
          (click to restore){/if}</span
      >
      <div class="tw-win">
        <button onclick={toggleMin} title="Minimize" aria-label="Minimize">-</button>
        <button onclick={toggleMax} title={maximized ? 'Restore' : 'Maximize'} aria-label="Maximize"
          >▢</button
        >
        <button onclick={closeAll} title="Close" aria-label="Close">×</button>
      </div>
    </div>

    {#if !minimized}
      <div class="tw-tabs">
        {#each tabs as t (t.id)}
          <div
            class="tw-tab"
            class:active={t.id === activeId}
            onclick={() => switchTo(t.id)}
            role="tab"
            tabindex="0"
            onkeydown={(e) => {
              if (e.key === 'Enter') switchTo(t.id);
            }}
          >
            <span class="tw-dot" class:on={t.connected}></span>
            {#if editingId === t.id}
              <input
                class="tt-input"
                bind:value={editLabel}
                onblur={() => finishRename(t)}
                onkeydown={(e) => {
                  if (e.key === 'Enter') finishRename(t);
                  if (e.key === 'Escape') editingId = null;
                }}
                onclick={(e) => e.stopPropagation()}
              />
            {:else}
              <span
                class="tw-label"
                ondblclick={(e) => {
                  e.stopPropagation();
                  startRename(t);
                }}>{t.label}</span
              >
            {/if}
            <button
              class="tw-x"
              onclick={(e) => {
                e.stopPropagation();
                closeTab(t.id);
              }}
              title="Close tab"
              aria-label="Close tab">×</button
            >
          </div>
        {/each}
        <button class="tw-add" onclick={addTab} title="New tab" aria-label="New tab">+</button>
        <span class="tw-spacer"></span>
        <div class="tw-font">
          <button onclick={() => setFont(-1)} title="Smaller font" aria-label="Smaller font"
            >−</button
          >
          <span class="tw-fs">{fontSize}px</span>
          <button onclick={() => setFont(1)} title="Larger font" aria-label="Larger font">+</button>
        </div>
      </div>
    {/if}

    <div class="tw-body" class:hidden={minimized} bind:this={containerEl}></div>

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
  /* Minimized: dock to the bottom as a title-bar-only bar instead of shrinking in place. */
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
  /* Mobile: a draggable/resizable floating window is useless on a phone, so the
     default (non-min) state fills the screen. */
  @media (max-width: 768px) {
    .tw:not(.min) {
      left: 0 !important;
      top: 0 !important;
      width: 100vw !important;
      height: 100dvh !important;
      border-radius: 0 !important;
      border-width: 0 !important;
    }
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
    font-size: var(--font-size-xs);
    color: var(--text-muted);
    font-family: var(--font-mono, monospace);
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
    font-size: var(--font-size-base);
    line-height: 1;
    border-radius: 4px;
  }
  .tw-win button:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }

  .tw-tabs {
    display: flex;
    align-items: center;
    gap: 1px;
    background: var(--bg-secondary);
    padding: 2px 4px 0;
    border-bottom: 1px solid var(--border-primary);
    flex-shrink: 0;
    overflow-x: auto;
    scrollbar-width: none;
  }
  .tw-tabs::-webkit-scrollbar {
    display: none;
  }
  .tw-tab {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 5px 10px;
    font-size: var(--font-size-xs);
    color: var(--text-muted);
    cursor: pointer;
    border-radius: 6px 6px 0 0;
    user-select: none;
    flex: 0 0 auto;
  }
  .tw-tab:hover {
    background: var(--bg-hover);
    color: var(--text-secondary);
  }
  .tw-tab.active {
    background: var(--bg-primary);
    color: var(--text-primary);
    border-bottom: 2px solid var(--accent);
  }
  .tw-dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--text-dim);
  }
  .tw-dot.on {
    background: var(--color-green);
  }
  .tw-x {
    background: none;
    border: none;
    color: inherit;
    cursor: pointer;
    padding: 0 2px;
    line-height: 1;
    opacity: 0.5;
    font-size: var(--font-size-sm);
  }
  .tw-x:hover {
    opacity: 1;
  }
  .tt-input {
    background: var(--bg-hover);
    border: 1px solid var(--accent);
    color: var(--text-primary);
    font-size: var(--font-size-xs);
    padding: 1px 4px;
    width: 84px;
    border-radius: 3px;
    outline: none;
  }
  .tw-add {
    background: none;
    border: none;
    color: var(--text-muted);
    font-size: var(--font-size-base);
    cursor: pointer;
    padding: 2px 9px;
    border-radius: 4px;
    flex: 0 0 auto;
  }
  .tw-add:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
  }
  .tw-spacer {
    flex: 1;
  }
  .tw-font {
    display: flex;
    align-items: center;
    gap: 4px;
    flex: 0 0 auto;
    padding-right: 4px;
  }
  .tw-font button {
    background: none;
    border: 1px solid var(--border-primary);
    color: var(--text-muted);
    cursor: pointer;
    width: 20px;
    height: 20px;
    border-radius: 3px;
    line-height: 1;
  }
  .tw-font button:hover {
    background: var(--bg-hover);
    color: var(--text-primary);
    border-color: var(--accent);
  }
  .tw-fs {
    font-size: var(--font-size-2xs);
    color: var(--text-muted);
    min-width: 30px;
    text-align: center;
  }

  /* No padding here; the terminal element insets itself to match the scrollbar gutter. */
  .tw-body {
    flex: 1;
    position: relative;
    min-height: 0;
    padding: 0;
    background: var(--bg-primary);
  }
  .tw-body.hidden {
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

  /* Thin overlay scrollbar so the right gutter (~8px) matches the left inset. */
  :global(.tw .xterm-viewport::-webkit-scrollbar) {
    width: 8px;
  }
  :global(.tw .xterm-viewport::-webkit-scrollbar-track) {
    background: transparent;
  }
  :global(.tw .xterm-viewport::-webkit-scrollbar-thumb) {
    background: var(--border-primary);
    border-radius: 4px;
  }
  :global(.tw .xterm-viewport::-webkit-scrollbar-thumb:hover) {
    background: var(--text-dim);
  }
</style>
