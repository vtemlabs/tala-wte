<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government
  use require a license from VTEM Labs. The Software may not be copied or
  redistributed. See the LICENSE file.
-->
<script lang="ts">
  import '../app.css';
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { pb, system } from '$lib/api';
  import ToastContainer from '$lib/components/ToastContainer.svelte';
  import Terminal from '$lib/components/Terminal.svelte';
  import Footer from '$lib/components/Footer.svelte';

  let { children } = $props();
  let authChecked = $state(false);
  let sidebarOpen = $state(false);
  let terminalOpen = $state(false);
  // Build version + update availability, shown in the footer and as a dot on Settings.
  let appVersion = $state('0.1.0');
  let updateAvailable = $state(false);
  let mode = $state('ap'); // 'ap' or 'client'
  // Desktop-only icon rail; persisted so the choice survives reloads.
  let collapsed = $state(false);

  function toggleCollapse() {
    collapsed = !collapsed;
    try {
      localStorage.setItem('sidebarCollapsed', collapsed ? '1' : '0');
    } catch {
      /* ignore */
    }
  }

  // Inline line-icon set (currentColor, 24x24).
  const ICONS: Record<string, string> = {
    dashboard:
      '<rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/>',
    networks:
      '<path d="M2 8.8a14 14 0 0 1 20 0"/><path d="M5 12.5a9 9 0 0 1 14 0"/><path d="M8.5 16a4.5 4.5 0 0 1 7 0"/><circle cx="12" cy="19.5" r="1"/>',
    portal:
      '<path d="M12 3l7 2.5V11c0 4.4-3 7.7-7 9-4-1.3-7-4.6-7-9V5.5L12 3z"/><path d="M9 12l2 2 4-4"/>',
    captures: '<path d="M3 12h3.5l2.5 7 4-15 2.5 8H21"/>',
    data: '<ellipse cx="12" cy="5.5" rx="7.5" ry="3"/><path d="M4.5 5.5v6c0 1.66 3.36 3 7.5 3s7.5-1.34 7.5-3v-6"/><path d="M4.5 11.5v6c0 1.66 3.36 3 7.5 3s7.5-1.34 7.5-3v-6"/>',
    ldap: '<circle cx="9" cy="8" r="3.2"/><path d="M3.5 20a5.5 5.5 0 0 1 11 0"/><path d="M16 5a3.2 3.2 0 0 1 0 6"/><path d="M20.5 20a5.5 5.5 0 0 0-4.3-5.4"/>',
    radius:
      '<circle cx="8" cy="14" r="4"/><path d="M10.8 11.2 20 2"/><path d="m16 6 2.5 2.5"/><path d="m14 8 2.5 2.5"/>',
    cert: '<circle cx="12" cy="9" r="5.2"/><path d="m9 13.4-1 7.6 4-2 4 2-1-7.6"/>',
    den: '<circle cx="12" cy="5" r="2.6"/><circle cx="5" cy="18" r="2.6"/><circle cx="19" cy="18" r="2.6"/><path d="M11 7.3 6.5 15.6M13 7.3l4.5 8.3"/>',
    settings:
      '<circle cx="12" cy="12" r="3.2"/><path d="M19.4 13.5a1.7 1.7 0 0 0 .3 1.9l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-2.9 1.2V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-2.9-1.2l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0-1.2-2.9H3a2 2 0 1 1 0-4h.1A1.7 1.7 0 0 0 4.3 7l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.9.3 1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.9-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.9 1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z"/>',
    terminal:
      '<rect x="3" y="4" width="18" height="16" rx="2"/><path d="m7.5 9.5 3 2.5-3 2.5"/><path d="M13 15h4"/>',
    logout:
      '<path d="M14 4h3a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2h-3"/><path d="M10 12H3"/><path d="m6.5 8-4 4 4 4"/>'
  };

  onMount(async () => {
    try {
      collapsed = localStorage.getItem('sidebarCollapsed') === '1';
    } catch {
      /* ignore */
    }
    // Detect AP vs client mode first (public endpoint) so the chrome and landing
    // match the role even right after login, when the layout does not remount.
    // The role redirect is handled reactively below.
    try {
      const s = await fetch('/api/wte/system/status').then((r) => r.json());
      if (s?.mode) mode = s.mode;
    } catch {
      /* default to AP nav */
    }

    const currentPath = window.location.pathname;
    if (currentPath === '/login') {
      authChecked = true;
      return;
    }

    if (!pb.authStore.isValid) {
      goto('/login');
      authChecked = true; // must be set so the login page can render
      return;
    }
    authChecked = true;

    // Resolve the running version and surface an update dot if a newer release
    // exists. The GitHub check can be slow, so it runs without blocking render.
    system
      .version()
      .then((v) => {
        if (v?.current) appVersion = v.current;
        updateAvailable = !!v?.update_available;
      })
      .catch(() => {
        /* offline or check failed; keep the default version */
      });
  });

  // Client mode hides the AP feature pages but keeps Dashboard, Traffic, Settings,
  // and Terminal. Bounce the AP-only routes (and "/") to the client dashboard.
  const CLIENT_ALLOWED = new Set(['/login', '/client', '/settings']);
  $effect(() => {
    if (!authChecked || mode !== 'client') return;
    const p = page.url.pathname;
    if (!CLIENT_ALLOWED.has(p) && !p.startsWith('/client/')) {
      goto('/client');
    }
  });

  function isActive(path: string): boolean {
    if (path === '/') return page.url.pathname === '/';
    return page.url.pathname.startsWith(path);
  }

  function navClick() {
    sidebarOpen = false;
  }
  const isLoginPage = $derived(page.url.pathname === '/login');
</script>

<svelte:head>
  <link rel="stylesheet" href="/fonts/fonts.css" />
  <title>Tala WTE</title>
  <meta name="description" content="Tala WTE - Wireless Training Environment" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
</svelte:head>

{#if !authChecked}
  <div
    style="display:flex;align-items:center;justify-content:center;min-height:100vh;background:var(--bg-primary);color:var(--text-dim)"
  >
    Loading...
  </div>
{:else if isLoginPage}
  {@render children()}
{:else}
  <div class="app-shell" class:sidebar-collapsed={collapsed} class:client-mode={mode === 'client'}>
    <button class="hamburger" onclick={() => (sidebarOpen = !sidebarOpen)} aria-label="Toggle menu">
      {sidebarOpen ? '×' : '≡'}
    </button>

    {#if sidebarOpen}
      <button
        class="sidebar-backdrop"
        onclick={() => (sidebarOpen = false)}
        aria-label="Close menu"
        tabindex="-1"
      ></button>
    {/if}

    {#snippet ic(name: string)}
      <svg
        class="nav-ic"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.7"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true">{@html ICONS[name] ?? ''}</svg
      >
    {/snippet}

    <nav class="sidebar" class:open={sidebarOpen}>
      <div class="sidebar-header">
        <img class="brand-full" src="/brand/tala-logo.png" alt="Tala" />
        <img class="brand-icon" src="/brand/tala-wolf.png" alt="Tala" />
        <span class="sidebar-sub">Wireless Training Environment</span>
      </div>

      <div class="sidebar-nav">
        {#if mode === 'client'}
          <a
            href="/client"
            class="nav-item"
            class:active={page.url.pathname === '/client'}
            onclick={navClick}
            title="Dashboard">{@render ic('dashboard')}<span>Dashboard</span></a
          >
          <a
            href="/client/traffic"
            class="nav-item"
            class:active={isActive('/client/traffic')}
            onclick={navClick}
            title="Traffic">{@render ic('captures')}<span>Traffic</span></a
          >
        {:else}
          <a
            href="/"
            class="nav-item"
            class:active={isActive('/')}
            onclick={navClick}
            title="Dashboard">{@render ic('dashboard')}<span>Dashboard</span></a
          >
          <div class="nav-group-label">Networks</div>
          <a
            href="/networks"
            class="nav-item"
            class:active={isActive('/networks')}
            onclick={navClick}
            title="Networks">{@render ic('networks')}<span>Networks</span></a
          >
          <a
            href="/portals"
            class="nav-item"
            class:active={isActive('/portals') &&
              !page.url.pathname.startsWith('/portals/captured')}
            onclick={navClick}
            title="Captive Portals">{@render ic('portal')}<span>Captive Portals</span></a
          >
          <a
            href="/den"
            class="nav-item"
            class:active={isActive('/den')}
            onclick={navClick}
            title="Den">{@render ic('den')}<span>Den</span></a
          >
          <div class="nav-group-label">Monitoring</div>
          <a
            href="/captures"
            class="nav-item"
            class:active={isActive('/captures')}
            onclick={navClick}
            title="Captures">{@render ic('captures')}<span>Captures</span></a
          >
          <a
            href="/portals/captured"
            class="nav-item"
            class:active={isActive('/portals/captured')}
            onclick={navClick}
            title="Captured Data">{@render ic('data')}<span>Captured Data</span></a
          >
          <div class="nav-group-label">Enterprise</div>
          <a
            href="/ldap"
            class="nav-item"
            class:active={isActive('/ldap')}
            onclick={navClick}
            title="LDAP Directory">{@render ic('ldap')}<span>LDAP Directory</span></a
          >
          <a
            href="/radius"
            class="nav-item"
            class:active={isActive('/radius')}
            onclick={navClick}
            title="RADIUS">{@render ic('radius')}<span>RADIUS</span></a
          >
          <a
            href="/certificates"
            class="nav-item"
            class:active={isActive('/certificates')}
            onclick={navClick}
            title="Certificates">{@render ic('cert')}<span>Certificates</span></a
          >
        {/if}
        <div class="nav-divider"></div>
        <a
          href="/settings"
          class="nav-item"
          class:active={isActive('/settings')}
          onclick={navClick}
          title="Settings"
          >{@render ic('settings')}<span>Settings</span>{#if updateAvailable}<span
              class="update-dot"
              title="Update available"
            ></span>{/if}</a
        >
      </div>

      <div class="sidebar-footer">
        <button
          class="nav-item logout-btn"
          onclick={() => {
            terminalOpen = true;
            sidebarOpen = false;
          }}
          title="Terminal">{@render ic('terminal')}<span>Terminal</span></button
        >
        <button
          class="nav-item logout-btn"
          onclick={() => {
            pb.authStore.clear();
            goto('/login');
          }}
          title="Logout">{@render ic('logout')}<span>Logout</span></button
        >
        <button
          class="nav-item logout-btn collapse-btn"
          onclick={toggleCollapse}
          title={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          aria-label="Toggle sidebar width"
        >
          <svg
            class="nav-ic chev"
            class:flip={collapsed}
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="1.7"
            stroke-linecap="round"
            stroke-linejoin="round"
            aria-hidden="true"><path d="m15 6-6 6 6 6" /></svg
          >
          <span>Collapse</span>
        </button>
        <span class="version">Tala WTE v{appVersion}</span>
      </div>
    </nav>

    <main class="content">
      {@render children()}
      <Footer />
    </main>
    <ToastContainer />
    <Terminal bind:open={terminalOpen} />
  </div>
{/if}

<style>
  .app-shell {
    display: flex;
    min-height: 100vh;
  }
  /* Client mode recolors the accent to orange so it is visually distinct from an
     AP server (blue) at a glance, using the existing palette. */
  .app-shell.client-mode {
    --accent: #f97316;
    --accent-hover: #fb923c;
    --accent-strong: #ea580c;
    --accent-soft: rgba(249, 115, 22, 0.14);
    --accent-glow: rgba(249, 115, 22, 0.4);
  }

  .hamburger {
    display: none;
    position: fixed;
    top: 10px;
    left: 10px;
    z-index: 300;
    background: var(--bg-secondary);
    border: 1px solid var(--border-primary);
    border-radius: 6px;
    color: var(--text-primary);
    font-size: 20px;
    width: 40px;
    height: 40px;
    cursor: pointer;
    line-height: 1;
    align-items: center;
    justify-content: center;
  }

  .sidebar-backdrop {
    display: none;
  }

  .sidebar {
    width: 236px;
    background: var(--bg-secondary);
    border-right: 1px solid var(--border-primary);
    display: flex;
    flex-direction: column;
    position: fixed;
    top: 0;
    left: 0;
    bottom: 0;
    z-index: 100;
    transition: width 0.16s ease;
  }
  .chev {
    transition: transform 0.16s ease;
  }
  .chev.flip {
    transform: rotate(180deg);
  }

  .sidebar-header {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 7px;
    padding: var(--space-xl);
    border-bottom: 1px solid var(--border-primary);
  }
  .brand-full {
    height: 30px;
    width: auto;
    display: block;
  }
  .brand-icon {
    display: none;
    width: 36px;
    height: 36px;
    flex-shrink: 0;
    border-radius: 9px;
    object-fit: contain;
    padding: 4px;
    background: var(--bg-tertiary);
    border: 1px solid var(--border-primary);
  }
  .sidebar-sub {
    font-size: 10px;
    color: var(--text-dim);
    letter-spacing: 0.04em;
    text-transform: uppercase;
  }

  .sidebar-nav {
    flex: 1;
    padding: var(--space-md) var(--space-md);
    overflow-y: auto;
  }

  .nav-group-label {
    padding: var(--space-md) var(--space-md) var(--space-xs);
    font-size: 10px;
    font-weight: 700;
    color: var(--text-dim);
    text-transform: uppercase;
    letter-spacing: 0.12em;
  }

  .nav-divider {
    height: 1px;
    background: var(--border-primary);
    margin: var(--space-sm) var(--space-md);
  }

  .nav-item {
    display: flex;
    align-items: center;
    gap: 11px;
    padding: 9px 12px;
    border-radius: var(--radius-md);
    font-size: var(--font-size-sm);
    font-weight: 500;
    color: var(--text-muted);
    transition: all var(--transition-fast);
    text-decoration: none;
    position: relative;
  }
  :global(.nav-ic) {
    width: 18px;
    height: 18px;
    flex-shrink: 0;
    opacity: 0.85;
  }
  .nav-item:hover {
    color: var(--text-primary);
    background: var(--bg-hover);
  }
  .nav-item.active {
    color: var(--text-primary);
    background: var(--accent-soft);
    box-shadow: inset 0 0 0 1px rgba(47, 129, 247, 0.22);
  }
  .nav-item.active :global(.nav-ic) {
    opacity: 1;
    color: var(--accent-hover);
  }
  .nav-item.active::before {
    content: '';
    position: absolute;
    left: -1px;
    top: 7px;
    bottom: 7px;
    width: 3px;
    border-radius: 0 3px 3px 0;
    background: var(--accent);
    box-shadow: 0 0 8px var(--accent-glow);
  }

  .sidebar-footer {
    padding: var(--space-md);
    border-top: 1px solid var(--border-primary);
  }
  .logout-btn {
    width: 100%;
    text-align: left;
    background: none;
    border: none;
    cursor: pointer;
    font-family: inherit;
    margin-bottom: 2px;
  }
  .version {
    display: block;
    font-size: 10px;
    color: var(--text-dim);
    font-family: var(--font-mono);
    margin-top: var(--space-sm);
    padding-left: 12px;
  }
  .update-dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    margin-left: auto;
    flex-shrink: 0;
    background: var(--accent);
    box-shadow: 0 0 6px var(--accent-glow);
  }
  .app-shell.sidebar-collapsed .update-dot {
    position: absolute;
    top: 7px;
    right: 9px;
    margin-left: 0;
  }

  .content {
    flex: 1;
    margin-left: 236px;
    padding: var(--space-2xl);
    min-height: 100vh;
    min-width: 0;
    transition: margin-left 0.16s ease;
  }

  /* Collapsed icon rail (desktop only). */
  @media (min-width: 901px) {
    .app-shell.sidebar-collapsed .sidebar {
      width: 64px;
    }
    .app-shell.sidebar-collapsed .content {
      margin-left: 64px;
    }
    .app-shell.sidebar-collapsed .sidebar-header {
      align-items: center;
      padding: var(--space-lg) 0;
    }
    .app-shell.sidebar-collapsed .brand-full,
    .app-shell.sidebar-collapsed .sidebar-sub,
    .app-shell.sidebar-collapsed .nav-group-label,
    .app-shell.sidebar-collapsed .version,
    .app-shell.sidebar-collapsed .nav-item > span {
      display: none;
    }
    .app-shell.sidebar-collapsed .brand-icon {
      display: block;
    }
    .app-shell.sidebar-collapsed .nav-item {
      justify-content: center;
      padding: 9px 0;
      gap: 0;
    }
    .app-shell.sidebar-collapsed .nav-divider {
      margin: var(--space-sm) var(--space-md);
    }
  }

  @media (max-width: 900px) {
    .hamburger {
      display: flex;
    }
    .sidebar-backdrop {
      display: block;
      position: fixed;
      inset: 0;
      background: rgba(0, 0, 0, 0.5);
      z-index: 99;
    }
    .sidebar {
      transform: translateX(-100%);
      transition: transform 0.2s ease;
      width: 240px;
      z-index: 200;
    }
    .sidebar.open {
      transform: translateX(0);
    }
    .content {
      margin-left: 0;
      padding: var(--space-lg);
      padding-top: 60px;
    }
  }
</style>
