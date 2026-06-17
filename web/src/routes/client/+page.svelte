<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, paid training, paid CTF,
  or any for-profit use requires a license from VTEM Labs. See the LICENSE file.
-->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { pb } from '$lib/api';
  import { toast } from '$lib/stores/toast';

  type ClientStatus = {
    connected: boolean;
    ssid: string;
    interface: string;
    ip: string;
    gateway: string;
    portal_state: string;
    generating: boolean;
    requests: number;
    bytes_rx: number;
    errors: number;
    last_error?: string;
    last_event?: string;
  };

  let config = $state<Record<string, any> | null>(null);
  let configName = $state('');
  let status = $state<ClientStatus | null>(null);
  let busy = $state('');
  let poll: ReturnType<typeof setInterval> | null = null;

  // Traffic toggles
  let web = $state(true);
  let dns = $state(true);
  let ping = $state(true);
  let downloads = $state(false);
  let creds = $state(false);
  let domainBait = $state(false);
  let local = $state(true);
  let internet = $state(true);

  // Operator-supplied target lists (one entry per line) and login credentials.
  let urlsText = $state('');
  let domainsText = $state('');
  let ipsText = $state('');
  let credsList = $state<{ url: string; username: string; password: string }[]>([]);

  const lines = (t: string): string[] =>
    t
      .split('\n')
      .map((s) => s.trim())
      .filter(Boolean);

  function addCred() {
    credsList = [...credsList, { url: '', username: '', password: '' }];
  }
  function removeCred(i: number) {
    credsList = credsList.filter((_, idx) => idx !== i);
  }

  function authHeaders(extra: Record<string, string> = {}): Record<string, string> {
    return pb.authStore.token ? { Authorization: pb.authStore.token, ...extra } : extra;
  }

  async function refresh() {
    try {
      status = await fetch('/api/wte/client/status', { headers: authHeaders() }).then((r) => r.json());
    } catch {
      /* ignore transient poll errors */
    }
  }

  onMount(() => {
    refresh();
    poll = setInterval(refresh, 2000);
  });
  onDestroy(() => poll && clearInterval(poll));

  async function onFile(e: Event) {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    try {
      config = JSON.parse(await file.text());
      configName = file.name;
      toast.success(`Imported config for "${config?.ssid ?? 'network'}"`);
    } catch {
      toast.err('That file is not a valid Tala WTE client config');
    }
  }

  async function connect() {
    if (!config) return;
    busy = 'connect';
    try {
      const r = await fetch('/api/wte/client/connect', {
        method: 'POST',
        headers: authHeaders({ 'Content-Type': 'application/json' }),
        body: JSON.stringify(config)
      });
      if (!r.ok) throw new Error((await r.json())?.error ?? 'connect failed');
      toast.success('Connecting to ' + config.ssid);
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to connect');
    }
    busy = '';
  }

  async function disconnect() {
    busy = 'disconnect';
    try {
      await fetch('/api/wte/client/disconnect', { method: 'POST', headers: authHeaders() });
      toast.success('Disconnected');
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to disconnect');
    }
    busy = '';
  }

  async function startTraffic() {
    busy = 'start';
    try {
      const r = await fetch('/api/wte/client/start', {
        method: 'POST',
        headers: authHeaders({ 'Content-Type': 'application/json' }),
        body: JSON.stringify({
          web,
          dns,
          ping,
          downloads,
          creds,
          domain: domainBait,
          local,
          internet,
          urls: lines(urlsText),
          domains: lines(domainsText),
          ips: lines(ipsText),
          credentials: credsList.filter((c) => c.url.trim())
        })
      });
      if (!r.ok) throw new Error((await r.json())?.error ?? 'start failed');
      toast.success('Traffic generation started');
      refresh();
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to start traffic');
    }
    busy = '';
  }

  async function stopTraffic() {
    busy = 'stop';
    try {
      await fetch('/api/wte/client/stop', { method: 'POST', headers: authHeaders() });
      toast.success('Traffic generation stopped');
      refresh();
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to stop traffic');
    }
    busy = '';
  }

  function fmtBytes(n: number): string {
    if (!n) return '0 B';
    const u = ['B', 'KB', 'MB', 'GB'];
    let i = 0;
    let v = n;
    while (v >= 1024 && i < u.length - 1) {
      v /= 1024;
      i++;
    }
    return `${v.toFixed(i ? 1 : 0)} ${u[i]}`;
  }

  const portalColor = $derived(
    status?.portal_state === 'passed'
      ? 'var(--color-green)'
      : status?.portal_state === 'failed'
        ? 'var(--color-red)'
        : 'var(--text-muted)'
  );
</script>

<svelte:head><title>Client - Tala WTE</title></svelte:head>

<div class="page-header">
  <div>
    <h1 class="page-title">Client Mode</h1>
    <p class="page-subtitle">Join a Tala WTE network and generate realistic traffic</p>
  </div>
</div>

<div class="panel section">
  <div class="stat-strip">
    <div class="stat-cell">
      <span class="k">Connection</span>
      <span class="v" style={status?.connected ? 'color:var(--color-green)' : ''}>
        {#if status?.connected}<span class="status-dot active"></span>{/if}{status?.connected
          ? 'Connected'
          : 'Offline'}
      </span>
    </div>
    <div class="stat-cell">
      <span class="k">SSID</span>
      <span class="v">{status?.ssid || '-'}</span>
    </div>
    <div class="stat-cell">
      <span class="k">IP / Gateway</span>
      <span class="v" style="font-size:var(--font-size-md)"
        >{status?.ip || '-'}{status?.gateway ? ' / ' + status.gateway : ''}</span
      >
    </div>
    <div class="stat-cell">
      <span class="k">Captive Portal</span>
      <span class="v" style="font-size:var(--font-size-md);color:{portalColor}"
        >{status?.portal_state ?? 'none'}</span
      >
    </div>
  </div>
</div>

<div class="split-main">
  <div class="panel">
    <div class="panel-head"><span class="panel-title">Network</span></div>
    <div class="panel-body stack">
      <div class="form-group">
        <label class="field-label" for="cfg">Import client config</label>
        <input class="input" id="cfg" type="file" accept=".json,application/json" onchange={onFile} />
        <span class="field-desc">Export a config from an access point (network detail page), then import it here.</span>
      </div>

      {#if config}
        <div class="cfg-summary">
          <div><span class="dim">SSID</span> <span class="mono">{config.ssid}</span></div>
          <div><span class="dim">Security</span> <span class="mono">{config.protocol}</span></div>
          <div><span class="dim">Captive portal</span> {config.portal?.enabled ? 'yes (auto-bypass)' : 'no'}</div>
        </div>
      {/if}

      <div class="btn-row">
        <button class="btn btn-primary" onclick={connect} disabled={!config || busy === 'connect'}>
          {busy === 'connect' ? 'Connecting…' : 'Connect'}
        </button>
        <button class="btn btn-secondary" onclick={disconnect} disabled={!status?.connected || busy === 'disconnect'}>
          Disconnect
        </button>
      </div>
    </div>
  </div>

  <div class="panel">
    <div class="panel-head"><span class="panel-title">Traffic Generation</span></div>
    <div class="panel-body stack">
      <div class="toggle-field">
        <div><div class="toggle-name">Web browsing</div><div class="field-desc">HTTP/HTTPS GETs</div></div>
        <input type="checkbox" bind:checked={web} />
      </div>
      <div class="toggle-field">
        <div><div class="toggle-name">DNS lookups</div><div class="field-desc">Background name resolution</div></div>
        <input type="checkbox" bind:checked={dns} />
      </div>
      <div class="toggle-field">
        <div><div class="toggle-name">Ping / local LAN</div><div class="field-desc">ICMP + intra-LAN chatter</div></div>
        <input type="checkbox" bind:checked={ping} />
      </div>
      <div class="toggle-field">
        <div><div class="toggle-name">Downloads / bandwidth</div><div class="field-desc">Periodic larger transfers</div></div>
        <input type="checkbox" bind:checked={downloads} />
      </div>
      <div class="toggle-field">
        <div><div class="toggle-name">Credential logins</div><div class="field-desc">Replays the logins below in cleartext (HTTP Basic + form POST) - capturable</div></div>
        <input type="checkbox" bind:checked={creds} />
      </div>
      <div class="toggle-field">
        <div><div class="toggle-name">Domain chatter (responder bait)</div><div class="field-desc">LLMNR / NBT-NS / mDNS name lookups for poisoning attacks</div></div>
        <input type="checkbox" bind:checked={domainBait} />
      </div>

      <div class="nav-divider"></div>

      <div class="toggle-field">
        <div><div class="toggle-name">Local targets</div><div class="field-desc">Gateway and LAN hosts</div></div>
        <input type="checkbox" bind:checked={local} />
      </div>
      <div class="toggle-field">
        <div><div class="toggle-name">Internet targets</div><div class="field-desc">Public sites and hosts</div></div>
        <input type="checkbox" bind:checked={internet} />
      </div>

      <div class="btn-row">
        <button class="btn btn-primary" onclick={startTraffic} disabled={!status?.connected || busy === 'start'}>
          {busy === 'start' ? 'Starting…' : 'Start traffic'}
        </button>
        <button class="btn btn-secondary" onclick={stopTraffic} disabled={!status?.generating || busy === 'stop'}>
          Stop
        </button>
      </div>
    </div>
  </div>
</div>

<div class="panel section">
  <div class="panel-head"><span class="panel-title">Targets &amp; Credentials</span></div>
  <div class="panel-body stack">
    <div class="grid3">
      <div class="form-group">
        <label class="field-label" for="urls">URLs to browse</label>
        <textarea class="input ta" id="urls" bind:value={urlsText} placeholder={"http://intranet.local/\nhttp://10.0.0.1/login"}></textarea>
        <span class="field-desc">One per line; used by the Web generator.</span>
      </div>
      <div class="form-group">
        <label class="field-label" for="domains">Domains to resolve</label>
        <textarea class="input ta" id="domains" bind:value={domainsText} placeholder={"intranet.local\nfileserver.corp"}></textarea>
        <span class="field-desc">One per line; used by DNS + domain chatter.</span>
      </div>
      <div class="form-group">
        <label class="field-label" for="ips">IPs to reach</label>
        <textarea class="input ta" id="ips" bind:value={ipsText} placeholder={"10.0.0.1\n10.0.0.50"}></textarea>
        <span class="field-desc">One per line; used by the Ping generator.</span>
      </div>
    </div>

    <div>
      <div class="creds-head">
        <span class="field-label" style="margin:0">Login credentials (cleartext, capturable)</span>
        <button class="btn btn-sm btn-secondary" onclick={addCred}>+ Add</button>
      </div>
      {#each credsList as c, i}
        <div class="cred-row">
          <input class="input" placeholder="http://target/login" bind:value={c.url} />
          <input class="input" placeholder="username" bind:value={c.username} />
          <input class="input" placeholder="password" bind:value={c.password} />
          <button class="btn btn-sm btn-danger" onclick={() => removeCred(i)} aria-label="Remove">×</button>
        </div>
      {/each}
      {#if credsList.length === 0}
        <span class="field-desc">Add logins the client replays; then enable "Credential logins" above.</span>
      {/if}
    </div>
  </div>
</div>

<div class="panel section">
  <div class="panel-head">
    <span class="panel-title">Live Stats</span>
    {#if status?.generating}<span class="count-pill" style="color:var(--color-green)">generating</span>{/if}
  </div>
  <div class="panel-body">
    <div class="stat-strip">
      <div class="stat-cell"><span class="k">Requests</span><span class="v">{status?.requests ?? 0}</span></div>
      <div class="stat-cell"><span class="k">Received</span><span class="v" style="font-size:var(--font-size-md)">{fmtBytes(status?.bytes_rx ?? 0)}</span></div>
      <div class="stat-cell"><span class="k">Errors</span><span class="v" style={status?.errors ? 'color:var(--color-yellow)' : ''}>{status?.errors ?? 0}</span></div>
    </div>
    {#if status?.last_event}<p class="event">{status.last_event}</p>{/if}
    {#if status?.last_error}<p class="event err">last error: {status.last_error}</p>{/if}
  </div>
</div>

<style>
  .cfg-summary {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: var(--space-md);
    background: var(--bg-input);
    border: 1px solid var(--border-primary);
    border-radius: var(--radius-sm);
    font-size: var(--font-size-sm);
  }
  .cfg-summary .dim {
    color: var(--text-muted);
    display: inline-block;
    width: 110px;
  }
  .mono {
    font-family: var(--font-mono);
    color: var(--text-primary);
  }
  .btn-row {
    display: flex;
    gap: var(--space-md);
  }
  .event {
    margin: var(--space-md) 0 0;
    font-size: var(--font-size-sm);
    color: var(--text-secondary);
  }
  .event.err {
    color: var(--color-yellow);
  }
  .grid3 {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: var(--space-lg);
  }
  .ta {
    min-height: 90px;
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    resize: vertical;
  }
  .creds-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: var(--space-sm);
  }
  .cred-row {
    display: grid;
    grid-template-columns: 2fr 1fr 1fr auto;
    gap: var(--space-sm);
    margin-bottom: var(--space-sm);
  }
</style>
