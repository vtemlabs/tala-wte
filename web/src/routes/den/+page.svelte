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
      await Promise.all([loadMembers(), loadNetworks()]);
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

  async function deploy(m: any) {
    const net = selectedNet[m.id];
    if (!net) {
      toast.err('Pick a network to deploy to');
      return;
    }
    busy = m.id;
    try {
      const r = await fetch(`/api/wte/den/${m.id}/deploy`, {
        method: 'POST',
        headers: authHeaders({ 'Content-Type': 'application/json' }),
        body: JSON.stringify({
          network_id: net,
          traffic: PROFILES[selectedProfile[m.id] || 'standard'].traffic,
          reconnect: PROFILES[selectedProfile[m.id] || 'standard'].reconnect
        })
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
    <button class="btn" onclick={updateAll} disabled={updating || members.length === 0}>
      {updating ? 'Updating...' : 'Update all members'}
    </button>
    <button class="btn" onclick={() => (guideOpen = true)}>Guide</button>
  </div>
</div>

<GuideModal bind:open={guideOpen} title={GUIDES.den.title} doc={GUIDES.den.doc} />

<div class="split-main">
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
                <span class="mono dim">{m.address}</span>
              </div>
              <button
                class="action-btn del-btn"
                onclick={() => remove(m)}
                aria-label="Remove member">Del</button
              >
            </div>
            <div class="member-stat">
              {#if !st}
                Checking...
              {:else if !st.reachable}
                <span class="warn">unreachable{st.error ? ` (${st.error})` : ''}</span>
              {:else if (st.status?.adapters ?? 0) === 0}
                <span class="warn">no wireless adapter</span>
              {:else if st.status?.connected}
                Connected to <b>{st.status.ssid}</b> · {st.status.ip} · {st.status.requests ?? 0} requests
              {:else}
                Reachable · idle
              {/if}
            </div>
            <div class="member-actions">
              <select class="input" bind:value={selectedNet[m.id]}>
                <option value="">Select network...</option>
                {#each networkList as n}<option value={n.id}>{n.ssid}</option>{/each}
              </select>
              <select class="input" bind:value={selectedProfile[m.id]}>
                {#each Object.entries(PROFILES) as [key, p]}<option value={key}>{p.label}</option
                  >{/each}
              </select>
              <button
                class="action-btn btn-success"
                onclick={() => deploy(m)}
                disabled={busy === m.id}>Deploy</button
              >
              <button class="action-btn btn-danger" onclick={() => stop(m)} disabled={busy === m.id}
                >Stop</button
              >
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </div>

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
    padding: var(--space-md);
    border: 1px solid var(--border-primary);
    border-radius: var(--radius-sm);
    background: var(--bg-input);
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
  .member-stat {
    font-size: var(--font-size-sm);
    color: var(--text-secondary);
  }
  .member-stat .warn {
    color: var(--color-yellow);
  }
  .member-actions {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-sm);
  }
  .member-actions .input {
    flex: 1;
    min-width: 140px;
  }
</style>
