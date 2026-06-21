<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government
  use require a license from VTEM Labs. The Software may not be copied or
  redistributed. See the LICENSE file.
-->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { pb } from '$lib/api';
  import { toast } from '$lib/stores/toast';
  import LogWindow from '$lib/components/LogWindow.svelte';
  import GuideModal from '$lib/components/GuideModal.svelte';
  import { GUIDES } from '$lib/guides';

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
    cycling?: boolean;
    cycles?: number;
    last_error?: string;
    last_event?: string;
  };

  let savedConfigs = $state<any[]>([]);
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

  // Handshake-capture reconnect cycling. A single dropdown each for frequency and
  // jitter (preset values plus a Custom option) keeps the controls compact.
  const FREQ_PRESETS = [
    { v: '30', label: 'Every 30 seconds' },
    { v: '60', label: 'Every minute' },
    { v: '120', label: 'Every 2 minutes' },
    { v: '300', label: 'Every 5 minutes' },
    { v: '900', label: 'Every 15 minutes' },
    { v: '1800', label: 'Every 30 minutes' },
    { v: '3600', label: 'Every hour' },
    { v: 'custom', label: 'Custom...' }
  ];
  const JITTER_PRESETS = [
    { v: '0', label: 'None' },
    { v: '5', label: 'Up to 5 seconds' },
    { v: '15', label: 'Up to 15 seconds' },
    { v: '30', label: 'Up to 30 seconds' },
    { v: '60', label: 'Up to 1 minute' },
    { v: 'custom', label: 'Custom...' }
  ];
  let freqSel = $state('120');
  let jitterSel = $state('15');
  let freqValue = $state(2);
  let freqUnit = $state('m');
  let jitterValue = $state(15);
  let jitterUnit = $state('s');
  const unitSec = (u: string): number => (u === 'h' ? 3600 : u === 'm' ? 60 : 1);
  const freqSeconds = (): number =>
    freqSel === 'custom' ? freqValue * unitSec(freqUnit) : Number(freqSel);
  const jitterSeconds = (): number =>
    jitterSel === 'custom' ? jitterValue * unitSec(jitterUnit) : Number(jitterSel);

  // Operator-supplied target lists (one entry per line) and login credentials.
  let urlsText = $state('');
  let domainsText = $state('');
  let ipsText = $state('');
  let datasets = $state<any[]>([]);
  let datasetSel = $state('');
  function applyDataset() {
    const d = datasets.find((x) => x.id === datasetSel);
    if (!d) return;
    urlsText = d.urls || '';
    domainsText = d.domains || '';
    ipsText = d.ips || '';
  }
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

  let logOpen = $state(false);
  let guideOpen = $state(false);
  let logLines = $state<string[]>([]);

  async function refresh() {
    try {
      status = await fetch('/api/wte/client/status', { headers: authHeaders() }).then((r) =>
        r.json()
      );
    } catch {
      /* ignore transient poll errors */
    }
    if (logOpen) {
      try {
        logLines =
          (await fetch('/api/wte/client/logs', { headers: authHeaders() }).then((r) => r.json()))
            .lines ?? [];
      } catch {
        /* ignore */
      }
    }
  }

  onMount(() => {
    refresh();
    loadSaved();
    poll = setInterval(refresh, 2000);
  });
  onDestroy(() => poll && clearInterval(poll));

  let dragging = $state(false);
  async function loadSaved() {
    try {
      savedConfigs = await pb.collection('client_configs').getFullList({ sort: '-created' });
    } catch {
      /* collection empty or unavailable */
    }
    try {
      datasets = await pb.collection('traffic_datasets').getFullList({ sort: 'name' });
    } catch {
      /* none */
    }
  }
  // Uploading a config saves it as a reusable network rather than connecting now.
  async function loadFile(file: File | null | undefined) {
    if (!file) return;
    try {
      const cfg = JSON.parse(await file.text());
      await pb.collection('client_configs').create({
        ssid: cfg.ssid ?? 'network',
        protocol: cfg.protocol ?? 'open',
        passphrase: cfg.passphrase ?? '',
        band: cfg.band ?? '2.4',
        channel: cfg.channel ?? 0,
        hidden: !!cfg.hidden,
        identity: cfg.identity ?? '',
        eap_password: cfg.eap_password ?? '',
        portal_enabled: !!cfg.portal?.enabled,
        portal_username: cfg.portal?.username ?? '',
        portal_password: cfg.portal?.password ?? ''
      });
      toast.success(`Saved network "${cfg.ssid ?? 'network'}"`);
      await loadSaved();
    } catch {
      toast.err('That file is not a valid Tala WTE client config');
    }
  }
  function onFile(e: Event) {
    const input = e.target as HTMLInputElement;
    loadFile(input.files?.[0]);
    input.value = '';
  }
  function onDrop(e: DragEvent) {
    e.preventDefault();
    dragging = false;
    loadFile(e.dataTransfer?.files?.[0]);
  }

  async function connectTo(rec: any) {
    busy = 'connect:' + rec.id;
    try {
      const r = await fetch('/api/wte/client/connect', {
        method: 'POST',
        headers: authHeaders({ 'Content-Type': 'application/json' }),
        body: JSON.stringify({
          ssid: rec.ssid,
          protocol: rec.protocol,
          passphrase: rec.passphrase,
          band: rec.band,
          channel: rec.channel,
          hidden: rec.hidden,
          identity: rec.identity,
          eap_password: rec.eap_password,
          portal: {
            enabled: rec.portal_enabled,
            username: rec.portal_username,
            password: rec.portal_password
          }
        })
      });
      if (!r.ok) throw new Error((await r.json())?.error ?? 'connect failed');
      toast.success('Connecting to ' + rec.ssid);
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to connect');
    }
    busy = '';
  }

  async function deleteSaved(rec: any) {
    try {
      await pb.collection('client_configs').delete(rec.id);
      await loadSaved();
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to delete network');
    }
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

  async function applyCycle() {
    busy = 'cycle';
    try {
      const r = await fetch('/api/wte/client/reconnect', {
        method: 'POST',
        headers: authHeaders({ 'Content-Type': 'application/json' }),
        body: JSON.stringify({
          enabled: true,
          frequency_seconds: freqSeconds(),
          jitter_seconds: jitterSeconds()
        })
      });
      if (!r.ok) throw new Error((await r.json())?.error ?? 'failed');
      toast.success('Reconnect cycling started');
      refresh();
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to start cycling');
    }
    busy = '';
  }
  async function stopCycle() {
    busy = 'cycle';
    try {
      await fetch('/api/wte/client/reconnect', {
        method: 'POST',
        headers: authHeaders({ 'Content-Type': 'application/json' }),
        body: JSON.stringify({ enabled: false })
      });
      toast.success('Reconnect cycling stopped');
      refresh();
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to stop cycling');
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

<svelte:head><title>Traffic Console - Tala WTE</title></svelte:head>

<div class="page-header">
  <div>
    <h1 class="page-title">Traffic Console</h1>
    <p class="page-subtitle">Join a network and generate realistic traffic</p>
  </div>
  <div class="header-actions">
    <button class="btn" onclick={() => (guideOpen = true)}>Guide</button>
    <button class="btn" onclick={() => (logOpen = true)}>Live Log</button>
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

<div class="panel section">
  <div class="panel-head"><span class="panel-title">Network</span></div>
  <div class="panel-body stack">
    <label
      class="dropzone"
      class:dragging
      for="cfg"
      ondragover={(e) => {
        e.preventDefault();
        dragging = true;
      }}
      ondragleave={() => (dragging = false)}
      ondrop={onDrop}
    >
      <input
        class="file-hidden"
        id="cfg"
        type="file"
        accept=".json,application/json"
        onchange={onFile}
      />
      <span class="dz-title">Drop a client config here, or click to browse</span>
      <span class="dz-sub"
        >Upload configs exported from access points; each is saved below to reuse anytime.</span
      >
    </label>

    <div class="saved-head">
      <span class="sub-label" style="margin:0">Saved networks</span>
      {#if status?.connected}
        <button class="action-btn btn-danger" onclick={disconnect} disabled={busy === 'disconnect'}>
          Disconnect
        </button>
      {/if}
    </div>

    {#if savedConfigs.length}
      <div class="saved-list">
        {#each savedConfigs as rec}
          {@const isConnected = !!status?.connected && status?.ssid === rec.ssid}
          <div class="saved-row" class:connected={isConnected}>
            <div class="saved-meta">
              <span class="mono saved-ssid">{rec.ssid}</span>
              <span class="saved-proto"
                >{(rec.protocol || 'open').replace('_', '-').toUpperCase()}</span
              >
              {#if isConnected}<span class="saved-badge">connected</span>{/if}
            </div>
            <div class="saved-actions">
              <button
                class="action-btn"
                class:btn-success={!isConnected}
                onclick={() => connectTo(rec)}
                disabled={busy === 'connect:' + rec.id || isConnected}
              >
                {busy === 'connect:' + rec.id
                  ? 'Connecting...'
                  : isConnected
                    ? 'Connected'
                    : 'Connect'}
              </button>
              <button
                class="action-btn del-btn"
                onclick={() => deleteSaved(rec)}
                aria-label="Delete network">Del</button
              >
            </div>
          </div>
        {/each}
      </div>
    {:else}
      <span class="field-desc">No saved networks yet. Upload a config above to save it here.</span>
    {/if}
  </div>
</div>

<div class="panel section">
  <div class="panel-head">
    <span class="panel-title">Traffic Generation</span>
    {#if status?.generating}<span class="count-pill" style="color:var(--color-green)"
        >generating</span
      >{/if}
  </div>
  <div class="panel-body">
    <div class="sub-label">Traffic types</div>
    <div class="toggle-grid">
      <div class="toggle-field">
        <div>
          <div class="toggle-name">Web browsing</div>
          <div class="field-desc">HTTP/HTTPS GETs</div>
        </div>
        <input type="checkbox" bind:checked={web} />
      </div>
      <div class="toggle-field">
        <div>
          <div class="toggle-name">DNS lookups</div>
          <div class="field-desc">Background name resolution</div>
        </div>
        <input type="checkbox" bind:checked={dns} />
      </div>
      <div class="toggle-field">
        <div>
          <div class="toggle-name">Ping / local LAN</div>
          <div class="field-desc">ICMP + intra-LAN chatter</div>
        </div>
        <input type="checkbox" bind:checked={ping} />
      </div>
      <div class="toggle-field">
        <div>
          <div class="toggle-name">Downloads / bandwidth</div>
          <div class="field-desc">Periodic larger transfers</div>
        </div>
        <input type="checkbox" bind:checked={downloads} />
      </div>
      <div class="toggle-field">
        <div>
          <div class="toggle-name">Credential logins</div>
          <div class="field-desc">
            Replays the logins below in cleartext (HTTP Basic + form POST) - capturable
          </div>
        </div>
        <input type="checkbox" bind:checked={creds} />
      </div>
      <div class="toggle-field">
        <div>
          <div class="toggle-name">Domain chatter (responder bait)</div>
          <div class="field-desc">LLMNR / NBT-NS / mDNS name lookups for poisoning attacks</div>
        </div>
        <input type="checkbox" bind:checked={domainBait} />
      </div>
    </div>
    <div class="sub-label">Target scope</div>
    <div class="toggle-grid">
      <div class="toggle-field">
        <div>
          <div class="toggle-name">Local targets</div>
          <div class="field-desc">Gateway and LAN hosts</div>
        </div>
        <input type="checkbox" bind:checked={local} />
      </div>
      <div class="toggle-field">
        <div>
          <div class="toggle-name">Internet targets</div>
          <div class="field-desc">Public sites and hosts</div>
        </div>
        <input type="checkbox" bind:checked={internet} />
      </div>
    </div>
    <div class="btn-row" style="margin-top:var(--space-lg)">
      <button
        class="btn btn-primary"
        onclick={startTraffic}
        disabled={!status?.connected || busy === 'start'}
      >
        {busy === 'start' ? 'Starting...' : 'Start traffic'}
      </button>
      <button
        class="btn btn-secondary"
        onclick={stopTraffic}
        disabled={!status?.generating || busy === 'stop'}
      >
        Stop
      </button>
    </div>
  </div>
</div>

<div class="panel section">
  <div class="panel-head">
    <span class="panel-title">Handshake Capture</span>
    {#if status?.cycling}<span class="count-pill" style="color:var(--color-green)"
        >cycling · {status?.cycles ?? 0}</span
      >{/if}
  </div>
  <div class="panel-body stack">
    <span class="field-desc"
      >Periodically deauth and reassociate so students can capture a fresh WPA handshake each cycle.</span
    >
    <div class="cycle-grid">
      <div class="form-group">
        <label class="field-label" for="freq">Frequency</label>
        <select class="input" id="freq" bind:value={freqSel}>
          {#each FREQ_PRESETS as p}<option value={p.v}>{p.label}</option>{/each}
        </select>
        {#if freqSel === 'custom'}
          <div class="dur-row">
            <input class="input dur-num" type="number" min="1" bind:value={freqValue} />
            <select class="input" bind:value={freqUnit}>
              <option value="s">seconds</option>
              <option value="m">minutes</option>
              <option value="h">hours</option>
            </select>
          </div>
        {/if}
      </div>
      <div class="form-group">
        <label class="field-label" for="jit">Jitter</label>
        <select class="input" id="jit" bind:value={jitterSel}>
          {#each JITTER_PRESETS as p}<option value={p.v}>{p.label}</option>{/each}
        </select>
        {#if jitterSel === 'custom'}
          <div class="dur-row">
            <input class="input dur-num" type="number" min="0" bind:value={jitterValue} />
            <select class="input" bind:value={jitterUnit}>
              <option value="s">seconds</option>
              <option value="m">minutes</option>
              <option value="h">hours</option>
            </select>
          </div>
        {/if}
      </div>
    </div>
    <div class="btn-row">
      <button
        class="btn btn-primary"
        onclick={applyCycle}
        disabled={!status?.connected || busy === 'cycle'}
      >
        {status?.cycling ? 'Update cycling' : 'Start cycling'}
      </button>
      {#if status?.cycling}
        <button class="btn btn-secondary" onclick={stopCycle} disabled={busy === 'cycle'}
          >Stop cycling</button
        >
      {/if}
    </div>
  </div>
</div>

<div class="panel section">
  <div class="panel-head"><span class="panel-title">Targets &amp; Credentials</span></div>
  <div class="panel-body stack">
    <div class="form-group">
      <label class="field-label" for="dataset">Apply a traffic dataset</label>
      <select
        class="input"
        id="dataset"
        bind:value={datasetSel}
        onchange={applyDataset}
        disabled={datasets.length === 0}
      >
        <option value=""
          >{datasets.length ? 'Choose a dataset to fill the targets…' : 'No datasets'}</option
        >
        {#each datasets as d}<option value={d.id}>{d.name}</option>{/each}
      </select>
      <span class="field-desc"
        >Fills the fields below from a saved dataset; edit them after if you like.</span
      >
    </div>
    <div class="grid3">
      <div class="form-group">
        <label class="field-label" for="urls">URLs to browse</label>
        <textarea
          class="input ta"
          id="urls"
          bind:value={urlsText}
          placeholder="http://intranet.local/"
        ></textarea>
        <span class="field-desc">One per line; used by the Web generator.</span>
      </div>
      <div class="form-group">
        <label class="field-label" for="domains">Domains to resolve</label>
        <textarea
          class="input ta"
          id="domains"
          bind:value={domainsText}
          placeholder="intranet.local"
        ></textarea>
        <span class="field-desc">One per line; used by DNS + domain chatter.</span>
      </div>
      <div class="form-group">
        <label class="field-label" for="ips">IPs to reach</label>
        <textarea class="input ta" id="ips" bind:value={ipsText} placeholder="10.0.0.1"></textarea>
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
          <button class="btn btn-sm btn-danger" onclick={() => removeCred(i)} aria-label="Remove"
            >×</button
          >
        </div>
      {/each}
      {#if credsList.length === 0}
        <span class="field-desc"
          >Add logins the client replays; then enable "Credential logins" above.</span
        >
      {/if}
    </div>
  </div>
</div>

<div class="panel section">
  <div class="panel-head">
    <span class="panel-title">Live Stats</span>
    {#if status?.generating}<span class="count-pill" style="color:var(--color-green)"
        >generating</span
      >{/if}
  </div>
  <div class="panel-body">
    <div class="stat-strip">
      <div class="stat-cell">
        <span class="k">Requests</span><span class="v">{status?.requests ?? 0}</span>
      </div>
      <div class="stat-cell">
        <span class="k">Received</span><span class="v" style="font-size:var(--font-size-md)"
          >{fmtBytes(status?.bytes_rx ?? 0)}</span
        >
      </div>
      <div class="stat-cell">
        <span class="k">Errors</span><span
          class="v"
          style={status?.errors ? 'color:var(--color-yellow)' : ''}>{status?.errors ?? 0}</span
        >
      </div>
    </div>
    <p class="event dim">Open <strong>Live Log</strong> (top right) for full terminal output.</p>
  </div>
</div>

<LogWindow
  bind:open={logOpen}
  title="Client Log"
  lines={logLines}
  streaming={!!status?.connected || !!status?.generating}
/>
<GuideModal bind:open={guideOpen} title={GUIDES.traffic.title} doc={GUIDES.traffic.doc} />

<style>
  .saved-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .saved-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-sm);
  }
  .saved-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-md);
    padding: var(--space-sm) var(--space-md);
    border: 1px solid var(--border-primary);
    border-radius: var(--radius-sm);
    background: var(--bg-input);
  }
  .saved-row.connected {
    border-color: var(--accent);
  }
  .saved-meta {
    display: flex;
    align-items: center;
    gap: var(--space-md);
    min-width: 0;
  }
  .saved-ssid {
    color: var(--text-primary);
    font-weight: 600;
  }
  .saved-proto {
    font-size: var(--font-size-xs);
    color: var(--text-dim);
    letter-spacing: 0.04em;
  }
  .saved-badge {
    font-size: var(--font-size-xs);
    color: var(--accent-hover);
    border: 1px solid var(--accent);
    border-radius: 4px;
    padding: 1px 6px;
  }
  .saved-actions {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
  }
  .cycle-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
    gap: var(--space-lg);
  }
  .dur-row {
    display: flex;
    gap: var(--space-sm);
  }
  .dur-num {
    width: 90px;
    flex-shrink: 0;
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
  .dropzone {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 4px;
    text-align: center;
    padding: var(--space-xl);
    border: 1px dashed var(--border-primary);
    border-radius: var(--radius-md);
    background: var(--bg-input);
    cursor: pointer;
    transition:
      border-color var(--transition-base),
      background var(--transition-base);
  }
  .dropzone:hover {
    border-color: var(--accent);
  }
  .dropzone.dragging {
    border-color: var(--accent);
    background: var(--accent-soft);
  }
  .file-hidden {
    display: none;
  }
  .dz-title {
    font-size: var(--font-size-sm);
    font-weight: 600;
    color: var(--text-primary);
  }
  .dz-sub {
    font-size: var(--font-size-xs);
    color: var(--text-dim);
  }
  .sub-label {
    font-size: var(--font-size-xs);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-dim);
    margin: var(--space-lg) 0 var(--space-sm);
  }
  .sub-label:first-child {
    margin-top: 0;
  }
  .toggle-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
    gap: var(--space-md);
  }
</style>
