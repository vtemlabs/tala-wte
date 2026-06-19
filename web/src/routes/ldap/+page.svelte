<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government
  use require a license from VTEM Labs. The Software may not be copied or
  redistributed. See the LICENSE file.
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { ldap } from '$lib/api';
  import { toast } from '$lib/stores/toast';
  import type { LDAPUser, LDAPGroup, LDAPStatus, TestAuthResult } from '$lib/types';
  import GuideModal from '$lib/components/GuideModal.svelte';
  import { GUIDES } from '$lib/guides';

  let guideOpen = $state(false);
  let tab = $state<'users' | 'groups' | 'test'>('users');
  let users = $state<LDAPUser[]>([]);
  let groups = $state<LDAPGroup[]>([]);
  let ldapStatus = $state<LDAPStatus | null>(null);
  let loading = $state(true);
  let error = $state('');

  let provisioning = $state(false);
  let provisionResult = $state<any>(null);
  let showProvision = $state(false);
  let customCompany = $state('');
  let customDomain = $state('');
  let customCount = $state(15);
  // Default to the realistic mix; all-strong-random is opt-in.
  let customRandomPass = $state(false);
  let revealedPasswords = $state<Record<string, boolean>>({});

  // Hashed userPassword values are unrecoverable, so render them as "(hashed)".
  function passwordCell(
    p: string | undefined,
    _uid: string
  ): { hidden: string; revealed: string; copyable: boolean } {
    if (!p) return { hidden: '-', revealed: '-', copyable: false };
    if (p.startsWith('{')) return { hidden: '(hashed)', revealed: '(hashed)', copyable: false };
    const hidden = '•'.repeat(Math.min(p.length, 12));
    return { hidden, revealed: p, copyable: true };
  }

  function togglePassword(uid: string) {
    revealedPasswords[uid] = !revealedPasswords[uid];
    revealedPasswords = { ...revealedPasswords };
  }

  async function copyPassword(value: string) {
    try {
      await navigator.clipboard.writeText(value);
      toast.info('Password copied to clipboard');
    } catch {
      // Clipboard API can fail in non-secure contexts.
      toast.err('Could not copy - select and copy manually');
    }
  }

  async function provisionRandom() {
    if (
      !confirm(
        'This will wipe the current directory and generate a random company with users. Continue?'
      )
    )
      return;
    provisioning = true;
    error = '';
    provisionResult = null;
    try {
      provisionResult = await ldap.provisionRandom();
      await loadAll();
    } catch (e: any) {
      error = e?.message ?? 'Provisioning failed';
    }
    provisioning = false;
  }

  async function provisionDefault() {
    if (
      !confirm(
        'This will wipe the current directory and create the default ACME Corp directory. Continue?'
      )
    )
      return;
    provisioning = true;
    error = '';
    provisionResult = null;
    try {
      provisionResult = await ldap.provision({
        company_name: 'ACME Corp',
        domain: 'acmecorp.local',
        user_count: 15,
        random_passwords: false
      });
      await loadAll();
    } catch (e: any) {
      error = e?.message ?? 'Provisioning failed';
    }
    provisioning = false;
  }

  async function provisionCustom() {
    if (!customCompany.trim() || !customDomain.trim()) {
      error = 'Company name and domain are required';
      return;
    }
    if (
      !confirm(
        `This will wipe the current directory and create ${customCount} users for ${customCompany}. Continue?`
      )
    )
      return;
    provisioning = true;
    error = '';
    provisionResult = null;
    try {
      provisionResult = await ldap.provision({
        company_name: customCompany,
        domain: customDomain,
        user_count: customCount,
        random_passwords: customRandomPass
      });
      await loadAll();
      showProvision = false;
    } catch (e: any) {
      error = e?.message ?? 'Provisioning failed';
    }
    provisioning = false;
  }

  let newUID = $state('');
  let newCN = $state('');
  let newSN = $state('');
  let newMail = $state('');
  let newPass = $state('');
  let addingUser = $state(false);

  let newGroupCN = $state('');
  let addingGroup = $state(false);

  // Groups list: sortable by name or member count, filterable by group name or
  // member uid (so an operator can find which groups a user belongs to).
  let groupFilter = $state('');
  let groupSort = $state<'name' | 'members'>('name');
  let groupSortDir = $state<'asc' | 'desc'>('asc');

  // memberUID extracts the bare uid from a member DN ("uid=jsmith,ou=Users,..").
  const memberUID = (dn: string) => dn.replace(/^uid=/i, '').split(',')[0] || dn;

  const shownGroups = $derived.by(() => {
    const q = groupFilter.trim().toLowerCase();
    let list = groups.map((g) => ({ ...g, uids: (g.members ?? []).map(memberUID) }));
    if (q) {
      list = list.filter(
        (g) => g.cn.toLowerCase().includes(q) || g.uids.some((u) => u.toLowerCase().includes(q))
      );
    }
    const dir = groupSortDir === 'asc' ? 1 : -1;
    list.sort((a, b) =>
      groupSort === 'members'
        ? (a.uids.length - b.uids.length) * dir
        : a.cn.localeCompare(b.cn) * dir
    );
    return list;
  });

  function sortGroupsBy(key: 'name' | 'members') {
    if (groupSort === key) groupSortDir = groupSortDir === 'asc' ? 'desc' : 'asc';
    else {
      groupSort = key;
      groupSortDir = key === 'members' ? 'desc' : 'asc';
    }
  }

  async function deleteGroup(cn: string) {
    if (!confirm(`Delete group "${cn}"?`)) return;
    try {
      await ldap.deleteGroup(cn);
      groups = groups.filter((g) => g.cn !== cn);
    } catch (e: any) {
      toast.err(e?.message ?? 'Delete failed');
    }
  }

  let testUID = $state('');
  let testPass = $state('');
  let testResult = $state<TestAuthResult | null>(null);
  let testing = $state(false);

  onMount(async () => {
    await loadAll();
  });

  async function loadAll() {
    loading = true;
    try {
      [ldapStatus, { users: users }, { groups: groups }] = await Promise.all([
        ldap.status(),
        ldap.users(),
        ldap.groups()
      ]);
    } catch (e: any) {
      error = e?.message ?? 'Failed to load LDAP data';
    }
    loading = false;
  }

  async function createUser() {
    if (!newUID || !newCN || !newPass) return;
    addingUser = true;
    try {
      await ldap.createUser({
        uid: newUID,
        cn: newCN,
        sn: newSN,
        mail: newMail,
        password: newPass
      });
      newUID = newCN = newSN = newMail = newPass = '';
      await loadAll();
    } catch (e: any) {
      error = e?.message ?? 'Failed to create user';
    }
    addingUser = false;
  }

  async function deleteUser(uid: string) {
    if (!confirm(`Delete user ${uid}?`)) return;
    try {
      await ldap.deleteUser(uid);
      users = users.filter((u) => u.uid !== uid);
    } catch (e: any) {
      error = e?.message ?? 'Failed to delete user';
    }
  }

  async function createGroup() {
    if (!newGroupCN) return;
    addingGroup = true;
    try {
      await ldap.createGroup(newGroupCN);
      newGroupCN = '';
      await loadAll();
    } catch (e: any) {
      error = e?.message ?? 'Failed to create group';
    }
    addingGroup = false;
  }

  async function testAuth() {
    testing = true;
    testResult = null;
    try {
      testResult = await ldap.testAuth(testUID, testPass);
    } catch (e: any) {
      testResult = {
        success: false,
        message: e?.message ?? 'LDAP server unreachable - check that slapd is running'
      };
    }
    testing = false;
  }
</script>

<svelte:head><title>LDAP Directory - Tala WTE</title></svelte:head>

<div class="page-header">
  <div>
    <h1 class="page-title">LDAP Directory</h1>
    <p class="page-subtitle">Embedded OpenLDAP - enterprise wireless authentication users</p>
  </div>
  <div class="header-actions">
    <button class="btn" onclick={() => (guideOpen = true)}>Guide</button>
    {#if ldapStatus}
      <span class="badge {ldapStatus.running ? 'badge-success' : 'badge-error'}">
        slapd {ldapStatus.running ? 'running' : 'stopped'}
      </span>
    {/if}
  </div>
</div>

{#if error}
  <div class="error-toast"><span>{error}</span><button onclick={() => (error = '')}>×</button></div>
{/if}

<div class="stack">
  {#if ldapStatus}
    <div class="panel">
      <div class="stat-strip">
        <div class="stat-cell">
          <span class="k">Base DN</span><span class="v mono dn-v">{ldapStatus.base_dn}</span>
        </div>
        <div class="stat-cell">
          <span class="k">Bind DN</span><span class="v mono dn-v"
            >cn=admin,{ldapStatus.base_dn}</span
          >
        </div>
        <div class="stat-cell"><span class="k">Port</span><span class="v mono">3389</span></div>
        <div class="stat-cell">
          <span class="k">Users</span><span class="v">{users.length}</span>
        </div>
      </div>
    </div>
  {/if}

  <div class="panel">
    <div class="panel-head">
      <h2 class="panel-title">Directory Provisioning</h2>
      <div class="header-actions">
        <button class="btn btn-primary" onclick={provisionDefault} disabled={provisioning}>
          {provisioning ? 'Provisioning...' : 'Reset to Default (ACME Corp)'}
        </button>
        <button class="btn btn-primary" onclick={provisionRandom} disabled={provisioning}>
          {provisioning ? 'Provisioning...' : 'Generate Random Company'}
        </button>
        <button
          class="btn"
          class:active={showProvision}
          onclick={() => (showProvision = !showProvision)}>Custom</button
        >
      </div>
    </div>
    <div class="panel-body">
      <p class="section-desc" style="margin-bottom:0">
        Wipe and rebuild the entire LDAP directory with generated users, groups, and credentials.
      </p>

      {#if showProvision}
        <div class="prov-custom">
          <div class="prov-grid">
            <div class="field">
              <label class="field-label" for="pCompany">Company Name</label>
              <input
                class="input"
                id="pCompany"
                bind:value={customCompany}
                placeholder="e.g. Contoso Ltd"
              />
            </div>
            <div class="field">
              <label class="field-label" for="pDomain">Email Domain</label>
              <input
                class="input"
                id="pDomain"
                bind:value={customDomain}
                placeholder="e.g. contoso.local"
              />
            </div>
            <div class="field">
              <label class="field-label" for="pCount">Users</label>
              <input
                class="input"
                id="pCount"
                type="number"
                bind:value={customCount}
                min="1"
                max="50"
                style="width:80px"
              />
            </div>
            <button
              class="btn btn-primary"
              onclick={provisionCustom}
              disabled={provisioning || !customCompany.trim() || !customDomain.trim()}
            >
              Provision
            </button>
          </div>
          <div class="toggle-field" style="margin-top:var(--space-md)">
            <div>
              <div style="font-size:var(--font-size-sm);font-weight:500">
                All Strong Random Passwords
              </div>
              <div class="field-desc">
                On: every user gets a unique 12-char random password. Off (recommended): realistic
                corporate mix - ~40% weak (Password1!, Welcome123, etc), ~30% semi-personal
                (firstname+year), ~30% strong random.
              </div>
            </div>
            <input type="checkbox" bind:checked={customRandomPass} />
          </div>
        </div>
      {/if}

      {#if provisionResult}
        <div class="prov-result">
          <div class="prov-result-head">
            Provisioned: {provisionResult.company_name} ({provisionResult.users?.length} users)
          </div>
          <div class="table-wrap" style="max-height:300px;overflow-y:auto">
            <table class="table">
              <thead><tr><th>UID</th><th>Name</th><th>Email</th><th>Password</th></tr></thead>
              <tbody>
                {#each provisionResult.users ?? [] as u}
                  <tr>
                    <td data-label="UID" class="mono">{u.uid}</td>
                    <td data-label="Name">{u.cn}</td>
                    <td data-label="Email" class="dim">{u.mail}</td>
                    <td data-label="Password" class="mono">{u.password}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        </div>
      {/if}
    </div>
  </div>

  <div class="panel">
    <div class="tab-bar tab-bar-pad">
      <button class="tab" class:active={tab === 'users'} onclick={() => (tab = 'users')}>
        Users <span class="count-pill">{users.length}</span>
      </button>
      <button class="tab" class:active={tab === 'groups'} onclick={() => (tab = 'groups')}>
        Groups <span class="count-pill">{groups.length}</span>
      </button>
      <button class="tab" class:active={tab === 'test'} onclick={() => (tab = 'test')}
        >Test Auth</button
      >
    </div>

    {#if tab === 'users'}
      <div class="panel-body">
        <div class="create-bar">
          <div class="user-form">
            <div class="field">
              <label class="field-label" for="newUID">UID *</label>
              <input class="input" id="newUID" bind:value={newUID} placeholder="jdoe" />
            </div>
            <div class="field">
              <label class="field-label" for="newCN">CN (Full Name) *</label>
              <input class="input" id="newCN" bind:value={newCN} placeholder="John Doe" />
            </div>
            <div class="field">
              <label class="field-label" for="newSN">SN (Last Name)</label>
              <input class="input" id="newSN" bind:value={newSN} placeholder="Doe" />
            </div>
            <div class="field">
              <label class="field-label" for="newMail">Email</label>
              <input
                class="input"
                id="newMail"
                type="email"
                bind:value={newMail}
                placeholder="jdoe@tala.wte"
              />
            </div>
            <div class="field">
              <label class="field-label" for="newPass">Password *</label>
              <input
                class="input"
                id="newPass"
                type="password"
                bind:value={newPass}
                placeholder="••••••••"
              />
            </div>
            <button
              class="btn btn-primary"
              onclick={createUser}
              disabled={addingUser || !newUID || !newCN || !newPass}
            >
              {addingUser ? '…' : 'Add User'}
            </button>
          </div>
        </div>
      </div>

      {#if loading}
        <div class="empty-state"><p>Loading users…</p></div>
      {:else if users.length === 0}
        <div class="empty-state"><p>No users in directory</p></div>
      {:else}
        <div class="table-wrap">
          <table class="table">
            <thead
              ><tr><th>UID</th><th>CN</th><th>Email</th><th>Password</th><th>DN</th><th></th></tr
              ></thead
            >
            <tbody>
              {#each users as u}
                {@const pw = passwordCell(u.password, u.uid)}
                <tr>
                  <td data-label="UID" class="mono">{u.uid}</td>
                  <td data-label="CN">{u.cn}</td>
                  <td data-label="Email" class="dim">{u.mail || '-'}</td>
                  <td data-label="Password">
                    <span class="mono pw-cell"
                      >{revealedPasswords[u.uid] ? pw.revealed : pw.hidden}</span
                    >
                    {#if pw.copyable}
                      <button
                        class="action-btn pw-action"
                        onclick={() => togglePassword(u.uid)}
                        title={revealedPasswords[u.uid] ? 'Hide' : 'Show'}
                      >
                        {revealedPasswords[u.uid] ? 'Hide' : 'Show'}
                      </button>
                      <button
                        class="action-btn pw-action"
                        onclick={() => copyPassword(pw.revealed)}
                        title="Copy to clipboard"
                      >
                        Copy
                      </button>
                    {/if}
                  </td>
                  <td data-label="DN" class="mono dim" style="font-size:var(--font-size-xs)"
                    >{u.dn}</td
                  >
                  <td data-label="">
                    <button
                      class="action-btn"
                      style="color:var(--color-red)"
                      onclick={() => deleteUser(u.uid)}>Del</button
                    >
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    {:else if tab === 'groups'}
      <div class="panel-body">
        <div class="create-bar">
          <div class="group-form">
            <div class="field">
              <label class="field-label" for="newGroupCN">Group CN</label>
              <input
                class="input"
                id="newGroupCN"
                bind:value={newGroupCN}
                placeholder="e.g. wifi-users"
                style="width:250px"
              />
            </div>
            <button
              class="btn btn-primary"
              onclick={createGroup}
              disabled={addingGroup || !newGroupCN}
            >
              {addingGroup ? '…' : 'Create Group'}
            </button>
          </div>
        </div>

        {#if groups.length === 0}
          <div class="empty-state"><p>No groups in directory</p></div>
        {:else}
          <div class="grp-controls">
            <input
              class="input grp-filter"
              bind:value={groupFilter}
              placeholder="Filter by group or member..."
            />
            <span class="grp-count">{shownGroups.length} of {groups.length} groups</span>
          </div>
          <div class="panel" style="margin-top:var(--space-md)">
            <div class="table-wrap">
              <table class="table grp-table">
                <thead>
                  <tr>
                    <th class="sortable" onclick={() => sortGroupsBy('name')}>
                      Group{#if groupSort === 'name'}<span class="arrow"
                          >{groupSortDir === 'asc' ? '▲' : '▼'}</span
                        >{/if}
                    </th>
                    <th class="sortable num" onclick={() => sortGroupsBy('members')}>
                      Members{#if groupSort === 'members'}<span class="arrow"
                          >{groupSortDir === 'asc' ? '▲' : '▼'}</span
                        >{/if}
                    </th>
                    <th>Membership</th>
                    <th class="act"></th>
                  </tr>
                </thead>
                <tbody>
                  {#each shownGroups as g (g.cn)}
                    <tr>
                      <td><span class="grp-name">{g.cn}</span></td>
                      <td class="num"><span class="count-pill">{g.uids.length}</span></td>
                      <td>
                        {#if g.uids.length}
                          <div class="grp-members">
                            {#each g.uids as uid}<span class="uid-chip mono">{uid}</span>{/each}
                          </div>
                        {:else}<span class="dim">empty</span>{/if}
                      </td>
                      <td class="act">
                        <button class="action-btn grp-del" onclick={() => deleteGroup(g.cn)}
                          >Del</button
                        >
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          </div>
        {/if}
      </div>
    {:else if tab === 'test'}
      <div class="panel-body">
        <div class="test-form">
          <div class="field" style="margin-bottom:var(--space-md)">
            <label class="field-label" for="testUID">Username (UID)</label>
            <input class="input" id="testUID" bind:value={testUID} placeholder="jdoe" />
          </div>
          <div class="field" style="margin-bottom:var(--space-lg)">
            <label class="field-label" for="testPass">Password</label>
            <input
              class="input"
              id="testPass"
              type="password"
              bind:value={testPass}
              placeholder="••••••••"
            />
          </div>
          <button
            class="btn btn-primary"
            onclick={testAuth}
            disabled={testing || !testUID || !testPass}
            style="width:100%"
          >
            {testing ? 'Testing…' : 'Test Authentication'}
          </button>

          {#if testResult}
            <div class="test-result" class:ok={testResult.success} class:fail={!testResult.success}>
              <div class="test-result-head">
                {testResult.success ? '✓ Authentication Successful' : '✗ Authentication Failed'}
              </div>
              {#if testResult.dn}
                <div
                  class="mono dim"
                  style="font-size:var(--font-size-xs);margin-top:var(--space-xs)"
                >
                  {testResult.dn}
                </div>
              {/if}
              {#if testResult.message && !testResult.success}
                <div class="dim" style="font-size:var(--font-size-xs);margin-top:var(--space-xs)">
                  {testResult.message}
                </div>
              {/if}
            </div>
          {/if}
        </div>
      </div>
    {/if}
  </div>
</div>

<GuideModal bind:open={guideOpen} title={GUIDES.ldap.title} doc={GUIDES.ldap.doc} />

<style>
  /* Long DN values truncate instead of overflowing. */
  .stat-cell .v.dn-v {
    font-size: var(--font-size-sm);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 100%;
  }

  .prov-custom {
    margin-top: var(--space-lg);
    padding-top: var(--space-lg);
    border-top: 1px solid var(--border-primary);
  }
  .prov-grid {
    display: grid;
    grid-template-columns: 1fr 1fr auto auto;
    gap: var(--space-sm);
    align-items: end;
  }
  .prov-result {
    margin-top: var(--space-lg);
    padding-top: var(--space-lg);
    border-top: 1px solid var(--border-primary);
  }
  .prov-result-head {
    font-weight: 600;
    margin-bottom: var(--space-sm);
    color: var(--status-active);
    font-size: var(--font-size-sm);
  }

  .tab-bar-pad {
    padding: 0 var(--space-xl);
  }
  .create-bar {
    margin-bottom: 0;
  }

  .user-form {
    display: grid;
    grid-template-columns: repeat(5, minmax(0, 1fr)) auto;
    gap: var(--space-sm);
    align-items: end;
  }
  .group-form {
    display: flex;
    gap: var(--space-sm);
    align-items: flex-end;
    flex-wrap: wrap;
  }
  .grp-controls {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-md);
    flex-wrap: wrap;
    margin-top: var(--space-lg);
  }
  .grp-filter {
    width: 320px;
    max-width: 100%;
  }
  .grp-count {
    font-size: var(--font-size-xs);
    color: var(--text-muted);
  }
  .grp-table th.sortable {
    cursor: pointer;
    user-select: none;
    white-space: nowrap;
  }
  .grp-table th.sortable:hover {
    color: var(--accent-hover);
  }
  .grp-table th.num,
  .grp-table td.num {
    text-align: center;
    width: 1%;
    white-space: nowrap;
  }
  .grp-table th.act,
  .grp-table td.act {
    text-align: right;
    width: 1%;
  }
  .grp-table .arrow {
    margin-left: 4px;
    font-size: 9px;
    color: var(--accent);
  }
  .grp-name {
    font-weight: 600;
    color: var(--text-primary);
    white-space: nowrap;
  }
  .grp-members {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    max-height: 88px;
    overflow-y: auto;
  }
  .uid-chip {
    font-size: 10px;
    color: var(--text-secondary);
    background: var(--bg-elevated);
    border: 1px solid var(--border-primary);
    border-radius: var(--radius-sm);
    padding: 1px 6px;
  }
  .grp-del {
    color: var(--color-red);
    border-color: rgba(244, 63, 94, 0.3);
  }
  .grp-del:hover {
    background: var(--color-red);
    color: #fff;
    border-color: transparent;
  }

  .test-form {
    max-width: 400px;
  }
  .test-result {
    margin-top: var(--space-lg);
    padding: var(--space-md);
    border-radius: var(--radius-md);
    border: 1px solid var(--border-primary);
  }
  .test-result.ok {
    border-color: var(--status-active);
    background: rgba(34, 197, 94, 0.1);
  }
  .test-result.fail {
    border-color: var(--status-error);
    background: rgba(244, 63, 94, 0.1);
  }
  .test-result-head {
    font-weight: 600;
  }
  .test-result.ok .test-result-head {
    color: var(--status-active);
  }
  .test-result.fail .test-result-head {
    color: var(--status-error);
  }

  .pw-cell {
    display: inline-block;
    min-width: 140px;
    font-size: var(--font-size-xs);
    color: var(--text-primary);
    letter-spacing: 0.02em;
  }
  .pw-action {
    margin-left: var(--space-xs);
    font-size: 10px;
    padding: 2px 6px;
    color: var(--text-dim);
  }
  .pw-action:hover {
    color: var(--text-primary);
  }

  @media (max-width: 820px) {
    .prov-grid {
      grid-template-columns: 1fr 1fr;
    }
    .user-form {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
    .user-form > .btn {
      grid-column: 1 / -1;
    }
  }
</style>
