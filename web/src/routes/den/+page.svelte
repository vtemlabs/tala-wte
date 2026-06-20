<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government use
  require a license from VTEM Labs. See the LICENSE file.
-->
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { pb } from '$lib/api';
  import { toast } from '$lib/stores/toast';
  import GuideModal from '$lib/components/GuideModal.svelte';
  import { GUIDES } from '$lib/guides';

  let members = $state<any[]>([]);
  let networkList = $state<any[]>([]);
  let statuses = $state<Record<string, any>>({});
  let selectedNet = $state<Record<string, string>>({});
  let selectedProfile = $state<Record<string, string>>({});
  let selectedDataset = $state<Record<string, string>>({});
  let datasets = $state<any[]>([]);

  // Deploy profiles bundle the full traffic config (and optional reconnect cycling)
  // the leader pushes to a member, the same settings you'd set on a client by hand.
  const PROFILES: Record<
    string,
    { label: string; traffic: Record<string, boolean>; reconnect: any }
  > = {
    standard: {
      label: 'Standard traffic',
      traffic: {
        web: true,
        dns: true,
        ping: true,
        downloads: false,
        creds: false,
        domain: false,
        local: true,
        internet: true
      },
      reconnect: null
    },
    full: {
      label: 'Full traffic',
      traffic: {
        web: true,
        dns: true,
        ping: true,
        downloads: true,
        creds: true,
        domain: true,
        local: true,
        internet: true
      },
      reconnect: null
    },
    handshake: {
      label: 'Handshake capture',
      traffic: {
        web: true,
        dns: true,
        ping: true,
        downloads: false,
        creds: false,
        domain: false,
        local: true,
        internet: true
      },
      reconnect: { enabled: true, frequency_seconds: 120, jitter_seconds: 15 }
    }
  };
  let busy = $state('');
  let poll: ReturnType<typeof setInterval> | null = null;

  let name = $state('');
  let address = $state('');
  let agentKey = $state('');
  let adding = $state(false);
  let guideOpen = $state(false);
  let updating = $state(false);

  function authHeaders(extra: Record<string, string> = {}): Record<string, string> {
    return pb.authStore.token ? { Authorization: pb.authStore.token, ...extra } : extra;
  }

  async function loadMembers() {
    members = await pb.collection('den_members').getFullList({ sort: '-created' });
  }
  async function loadNetworks() {
    networkList = await pb.collection('networks').getFullList({ sort: 'ssid' });
  }
  async function loadDatasets() {
    datasets = await pb.collection('traffic_datasets').getFullList({ sort: 'name' });
  }
  const lines = (s: string): string[] =>
    (s || '')
      .split('\n')
      .map((x) => x.trim())
      .filter(Boolean);

  let dsName = $state('');
  let dsDesc = $state('');
  let dsUrls = $state('');
  let dsDomains = $state('');
  let dsIps = $state('');
  let dsEditId = $state('');
  let dsSaving = $state(false);

  function editDataset(d: any) {
    dsEditId = d.id;
    dsName = d.name;
    dsDesc = d.description || '';
    dsUrls = d.urls || '';
    dsDomains = d.domains || '';
    dsIps = d.ips || '';
  }
  function resetDsForm() {
    dsEditId = '';
    dsName = dsDesc = dsUrls = dsDomains = dsIps = '';
  }
  async function saveDataset() {
    if (!dsName.trim()) {
      toast.err('Dataset name is required');
      return;
    }
    dsSaving = true;
    try {
      const data = {
        name: dsName.trim(),
        description: dsDesc.trim(),
        urls: dsUrls,
        domains: dsDomains,
        ips: dsIps
      };
      if (dsEditId) await pb.collection('traffic_datasets').update(dsEditId, data);
      else await pb.collection('traffic_datasets').create({ ...data, type: 'custom' });
      resetDsForm();
      await loadDatasets();
      toast.success('Dataset saved');
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to save dataset');
    }
    dsSaving = false;
  }
  async function deleteDataset(d: any) {
    if (!confirm(`Delete dataset "${d.name}"?`)) return;
    try {
      await pb.collection('traffic_datasets').delete(d.id);
      if (dsEditId === d.id) resetDsForm();
      await loadDatasets();
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to delete dataset');
    }
  }
  async function refreshStatuses() {
    for (const m of members) {
      try {
        statuses[m.id] = await fetch(`/api/wte/den/${m.id}/status`, {
          headers: authHeaders()
        }).then((r) => r.json());
      } catch {
        statuses[m.id] = { reachable: false };
      }
    }
    statuses = { ...statuses };
  }

  onMount(async () => {
    try {
      await Promise.all([loadMembers(), loadNetworks(), loadDatasets()]);
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to load den');
    }
    refreshStatuses();
    poll = setInterval(refreshStatuses, 5000);
  });
  onDestroy(() => poll && clearInterval(poll));

  async function addMember() {
    if (!name.trim() || !address.trim() || !agentKey.trim()) {
      toast.err('Name, address, and agent key are all required');
      return;
    }
    adding = true;
    try {
      await pb.collection('den_members').create({
        name: name.trim(),
        address: address.trim(),
        agent_key: agentKey.trim()
      });
      name = '';
      address = '';
      agentKey = '';
      await loadMembers();
      refreshStatuses();
      toast.success('Member added to the den');
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to add member');
    }
    adding = false;
  }

  // Auto-discovery: find other Tala WTE instances on the LAN over mDNS so members
  // can be registered without knowing their address up front (handy for fresh
  // installs and hosts whose DHCP lease changes).
  let discovered = $state<any[]>([]);
  let scanning = $state(false);

  async function scanDiscovered() {
    scanning = true;
    try {
      const d = await fetch('/api/wte/den/discovered', { headers: authHeaders() }).then((r) =>
        r.json()
      );
      discovered = d.peers ?? [];
      if (discovered.length === 0) toast.info('No other Tala WTE instances found on the LAN');
    } catch (e: any) {
      toast.err(e?.message ?? 'Discovery failed');
    }
    scanning = false;
  }

  function useDiscovered(p: any) {
    name = p.name || name;
    address = p.address || p.host || address;
    toast.info('Filled the form - paste the member agent key, then Add member');
  }

  async function deploy(m: any) {
    const net = selectedNet[m.id];
    if (!net) {
      toast.err('Pick a network to deploy to');
      return;
    }
    busy = m.id;
    try {
      const profile = PROFILES[selectedProfile[m.id] || 'standard'];
      const ds = datasets.find((d) => d.id === selectedDataset[m.id]);
      const traffic: Record<string, any> = { ...profile.traffic };
      if (ds) {
        traffic.urls = lines(ds.urls);
        traffic.domains = lines(ds.domains);
        traffic.ips = lines(ds.ips);
      }
      const r = await fetch(`/api/wte/den/${m.id}/deploy`, {
        method: 'POST',
        headers: authHeaders({ 'Content-Type': 'application/json' }),
        body: JSON.stringify({ network_id: net, traffic, reconnect: profile.reconnect })
      });
      if (!r.ok) throw new Error((await r.json())?.error ?? 'deploy failed');
      toast.success(`Deploying ${m.name} to ${netName(net)}`);
      await loadMembers();
    } catch (e: any) {
      toast.err(e?.message ?? 'Deploy failed');
    }
    busy = '';
  }

  async function stop(m: any) {
    busy = m.id;
    try {
      await fetch(`/api/wte/den/${m.id}/stop`, { method: 'POST', headers: authHeaders() });
      toast.success(`Stopped ${m.name}`);
      await loadMembers();
    } catch (e: any) {
      toast.err(e?.message ?? 'Stop failed');
    }
    busy = '';
  }

  async function remove(m: any) {
    if (!confirm(`Remove ${m.name} from the den?`)) return;
    try {
      await pb.collection('den_members').delete(m.id);
      await loadMembers();
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to remove member');
    }
  }

  function netName(id: string): string {
    return networkList.find((n) => n.id === id)?.ssid ?? '';
  }

  // Update the whole pack: each member pulls and applies the latest release, then
  // restarts, so the leader and its members stay on matching versions.
  async function updateAll() {
    if (
      !confirm('Push the latest update to all den members? Each downloads, applies, and restarts.')
    )
      return;
    updating = true;
    try {
      const r = await fetch('/api/wte/den/update', { method: 'POST', headers: authHeaders() });
      const j = await r.json();
      if (!r.ok) throw new Error(j?.error ?? 'update failed');
      const results = j.results ?? [];
      const ok = results.filter((x: any) => x.ok).length;
      for (const x of results) if (!x.ok) toast.err(`${x.name}: ${x.detail}`);
      toast.success(
        `Update sent to ${ok}/${results.length} member${results.length === 1 ? '' : 's'}`
      );
      refreshStatuses();
    } catch (e: any) {
      toast.err(e?.message ?? 'Den update failed');
    }
    updating = false;
  }
</script>

<svelte:head><title>Den - Tala WTE</title></svelte:head>

<div class="page-header">
  <div>
    <h1 class="page-title">Den</h1>
    <p class="page-subtitle">Drive a pack of client members from the den leader</p>
  </div>
  <div class="header-actions">
    <button class="btn" onclick={() => (guideOpen = true)}>Guide</button>
    <button
      class="btn btn-primary"
      onclick={updateAll}
      disabled={updating || members.length === 0}
    >
      {updating ? 'Updating...' : 'Update all members'}
    </button>
  </div>
</div>

<GuideModal bind:open={guideOpen} title={GUIDES.den.title} doc={GUIDES.den.doc} />

<div class="stack">
  <div class="panel">
    <div class="panel-head">
      <span class="panel-title">Members</span>
      <span class="count-pill">{members.length}</span>
    </div>
    {#if members.length === 0}
      <div class="empty-state" style="padding:var(--space-2xl)">
        <p>No members yet. Add a client with its address and agent key.</p>
      </div>
    {:else}
      <div class="member-list">
        {#each members as m}
          {@const st = statuses[m.id]}
          {@const online = !!st?.reachable && !!st?.status?.connected}
          <div class="member">
            <div class="member-top">
              <div class="member-id">
                <span class="status-dot" class:active={online} class:inactive={!online}></span>
                <span class="member-name">{m.name}</span>
                <span class="mono dim member-addr">{m.address}</span>
              </div>
              <div class="member-top-right">
                {#if !st}
                  <span class="badge badge-neutral">checking</span>
                {:else if !st.reachable}
                  <span class="badge badge-error">unreachable</span>
                {:else if st.status?.radio_wedged}
                  <span class="badge badge-error">radio wedged</span>
                {:else if (st.status?.adapters ?? 0) === 0}
                  <span class="badge badge-warning">no adapter</span>
                {:else if st.status?.connected}
                  <span class="badge badge-success">connected</span>
                {:else}
                  <span class="badge badge-neutral">idle</span>
                {/if}
                <button class="action-btn del-btn" onclick={() => remove(m)} aria-label="Remove member"
                  >Del</button
                >
              </div>
            </div>

            <div class="member-detail">
              {#if !st}
                Checking the member…
              {:else if !st.reachable}
                <span class="member-err">{st.error || 'not reachable'}</span>
              {:else if st.status?.radio_wedged}
                <span class="member-err"
                  >Radio stopped responding (driver wedge). Power-cycle or replug the adapter.</span
                >
              {:else if st.status?.connected}
                Connected to <b>{st.status.ssid}</b> · {st.status.ip} · {st.status.requests ?? 0} requests
              {:else if st.status?.adapter_names?.length}
                <span class="mono">{st.status.adapter_names.join(', ')}</span>{#if st.status?.version}<span
                    class="dim"
                  >
                    · v{st.status.version}</span
                  >{/if}
              {:else}
                Reachable, idle
              {/if}
            </div>

            {#if st?.reachable && st.status?.adapter_limits?.length}
              <div class="member-limits">
                <span class="lim-label">Limits</span>
                {st.status.adapter_limits.join('; ')}
              </div>
            {/if}
            {#if st?.reachable && st.status?.connected && !m.network_id}
              <div class="member-note">In use by another pack leader</div>
            {/if}
            {#if st?.reachable && !st.status?.connected && st.status?.last_error}
              <div class="member-err">{st.status.last_error}</div>
            {/if}

            <div class="member-actions">
              <select class="input" bind:value={selectedNet[m.id]}>
                <option value="">Select network...</option>
                {#each networkList as n}<option value={n.id}>{n.ssid}</option>{/each}
              </select>
              <select class="input" bind:value={selectedProfile[m.id]}>
                {#each Object.entries(PROFILES) as [key, p]}<option value={key}>{p.label}</option>{/each}
              </select>
              <select class="input" bind:value={selectedDataset[m.id]} title="Traffic dataset (targets)">
                <option value="">Default targets</option>
                {#each datasets as d}<option value={d.id}>{d.name}</option>{/each}
              </select>
              <button class="btn btn-sm btn-success" onclick={() => deploy(m)} disabled={busy === m.id}
                >Deploy</button
              >
              <button class="btn btn-sm btn-danger" onclick={() => stop(m)} disabled={busy === m.id}
                >Stop</button
              >
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>

  <div class="grid grid-2">
    <div class="panel">
      <div class="panel-head"><span class="panel-title">Add Member</span></div>
    <div class="panel-body stack">
      <div class="form-group">
        <label class="field-label" for="mname">Name</label>
        <input class="input" id="mname" bind:value={name} placeholder="lab-client-1" />
      </div>
      <div class="form-group">
        <label class="field-label" for="maddr">Address</label>
        <input
          class="input"
          id="maddr"
          bind:value={address}
          placeholder="10.0.0.50 or client-host"
        />
        <span class="field-desc">Host or host:port; https and :8443 are assumed if omitted.</span>
      </div>
      <div class="form-group">
        <label class="field-label" for="mkey">Agent key</label>
        <input class="input" id="mkey" bind:value={agentKey} placeholder="paste from the member" />
        <span class="field-desc">Copy it from the member's Settings -> Den Agent Key.</span>
      </div>
      <button class="btn btn-primary" onclick={addMember} disabled={adding}>
        {adding ? 'Adding...' : 'Add member'}
      </button>
    </div>
  </div>

  <div class="panel">
    <div class="panel-head">
      <span class="panel-title">Discovered on LAN</span>
      <button class="btn" onclick={scanDiscovered} disabled={scanning}>
        {scanning ? 'Scanning...' : 'Scan'}
      </button>
    </div>
    <div class="panel-body stack">
      {#if discovered.length === 0}
        <p class="field-desc">
          Scan to find other Tala WTE instances advertising on this network - handy for fresh
          installs or members whose DHCP address changes. Pick one to fill the Add Member form,
          then paste its agent key.
        </p>
      {:else}
        <div class="member-list">
          {#each discovered as p}
            <div class="member">
              <div class="member-top">
                <div class="member-id">
                  <span class="member-name">{p.name}</span>
                  <span class="mono dim">{p.address || p.host}</span>
                </div>
                <button class="action-btn" onclick={() => useDiscovered(p)}>Use</button>
              </div>
              <div class="member-meta">
                {p.role || 'unknown'}{#if p.version}
                  · v{p.version}{/if}
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  </div>
</div>

  <div class="panel">
    <div class="panel-head">
      <span class="panel-title">Traffic Datasets</span>
      <span class="count-pill">{datasets.length}</span>
    </div>
    <div class="panel-body">
      <p class="section-desc">
        Reusable target lists a member's traffic generators browse, resolve, and ping. Pick one per
        member in the deploy row above; leave a member on "Default targets" to use the built-in safe
        pool.
      </p>
      {#if datasets.length}
        <div class="table-wrap">
          <table class="table">
            <thead>
              <tr><th>Name</th><th>Targets</th><th>Type</th><th class="actions-col"></th></tr>
            </thead>
            <tbody>
              {#each datasets as d}
                <tr>
                  <td>
                    {d.name}
                    {#if d.description}<div class="field-desc">{d.description}</div>{/if}
                  </td>
                  <td class="dim"
                    >{lines(d.urls).length} URLs · {lines(d.domains).length} domains · {lines(d.ips)
                      .length} IPs</td
                  >
                  <td>
                    <span class="badge {d.type === 'builtin' ? 'badge-info' : 'badge-neutral'}"
                      >{d.type || 'custom'}</span
                    >
                  </td>
                  <td class="actions-col">
                    <div class="row-actions">
                      <button class="action-btn" onclick={() => editDataset(d)}>Edit</button>
                      <button class="action-btn del-btn" onclick={() => deleteDataset(d)}>Del</button>
                    </div>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}

      <div class="section-title" style="margin-top:var(--space-xl)">
        {dsEditId ? 'Edit dataset' : 'New dataset'}
      </div>
      <div class="grid grid-2">
        <div class="form-group">
          <label class="field-label" for="dsName">Name</label>
          <input class="input" id="dsName" bind:value={dsName} placeholder="e.g. Marketing team" />
        </div>
        <div class="form-group">
          <label class="field-label" for="dsDesc">Description</label>
          <input
            class="input"
            id="dsDesc"
            bind:value={dsDesc}
            placeholder="What this profile simulates"
          />
        </div>
      </div>
      <div class="grid grid-3" style="margin-top:var(--space-md)">
        <div class="form-group">
          <label class="field-label" for="dsUrls">URLs to browse</label>
          <textarea class="input" id="dsUrls" rows="4" bind:value={dsUrls} placeholder="one per line"
          ></textarea>
        </div>
        <div class="form-group">
          <label class="field-label" for="dsDomains">Domains to resolve</label>
          <textarea
            class="input"
            id="dsDomains"
            rows="4"
            bind:value={dsDomains}
            placeholder="one per line"
          ></textarea>
        </div>
        <div class="form-group">
          <label class="field-label" for="dsIps">IPs to ping</label>
          <textarea class="input" id="dsIps" rows="4" bind:value={dsIps} placeholder="one per line"
          ></textarea>
        </div>
      </div>
      <div class="header-actions" style="margin-top:var(--space-md)">
        <button class="btn btn-primary" onclick={saveDataset} disabled={dsSaving || !dsName}
          >{dsSaving ? 'Saving...' : dsEditId ? 'Update dataset' : 'Add dataset'}</button
        >
        {#if dsEditId}<button class="btn" onclick={resetDsForm}>Cancel</button>{/if}
      </div>
    </div>
  </div>
</div>

<style>
  .member-list {
    display: flex;
    flex-direction: column;
    padding: var(--space-sm) var(--space-lg) var(--space-lg);
    gap: var(--space-md);
  }
  .member {
    display: flex;
    flex-direction: column;
    gap: var(--space-sm);
    padding: var(--space-md) var(--space-lg);
    border: 1px solid var(--border-primary);
    border-radius: var(--radius-md);
    background: var(--bg-tertiary);
  }
  .member-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-md);
  }
  .member-id {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
    min-width: 0;
  }
  .member-name {
    color: var(--text-primary);
    font-weight: 600;
  }
  .member-addr {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .member-top-right {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
    flex-shrink: 0;
  }
  .member-detail {
    font-size: var(--font-size-sm);
    color: var(--text-secondary);
  }
  .member-limits {
    font-size: var(--font-size-xs);
    color: var(--text-muted);
  }
  .lim-label {
    font-size: var(--font-size-2xs);
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-dim);
    margin-right: var(--space-xs);
  }
  .member-err {
    font-size: var(--font-size-xs);
    color: var(--color-red);
  }
  .member-note {
    font-size: var(--font-size-xs);
    color: var(--color-cyan);
  }
  .member-meta {
    font-size: var(--font-size-xs);
    color: var(--text-muted);
    margin-top: 2px;
  }
  .member-actions {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-sm);
    margin-top: var(--space-xs);
  }
  .member-actions .input {
    flex: 1;
    min-width: 140px;
  }

</style>
