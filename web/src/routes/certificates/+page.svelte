<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government
  use require a license from VTEM Labs. The Software may not be copied or
  redistributed. See the LICENSE file.
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { certificates } from '$lib/api';
  import { toast } from '$lib/stores/toast';
  import GuideModal from '$lib/components/GuideModal.svelte';
  import { GUIDES } from '$lib/guides';

  let guideOpen = $state(false);
  let certs = $state<Record<string, any>[]>([]);
  let loading = $state(true);
  let creating = $state(false);
  let error = $state('');
  let newName = $state('');
  let newType = $state<'ca' | 'server' | 'client'>('server');
  let newUID = $state('');

  onMount(async () => {
    try {
      certs = await certificates.list();
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to load certificates');
    } finally {
      loading = false;
    }
  });

  async function createCA() {
    creating = true;
    error = '';
    try {
      await certificates.createCA();
      certs = await certificates.list();
    } catch (e: any) {
      error = e?.message ?? 'Failed to create CA';
    }
    creating = false;
  }

  async function createCert() {
    creating = true;
    error = '';
    try {
      if (newType === 'server') await certificates.createServer(newName);
      else if (newType === 'client') await certificates.createClient(newUID || newName);
      certs = await certificates.list();
      newName = '';
      newUID = '';
    } catch (e: any) {
      error = e?.message ?? 'Failed to create cert';
    }
    creating = false;
  }
</script>

<svelte:head><title>Certificates - Tala WTE</title></svelte:head>

<div class="page-header">
  <div>
    <h1 class="page-title">Certificates</h1>
    <p class="page-subtitle">CA and TLS certificates for WPA-Enterprise networks</p>
  </div>
  <div class="header-actions">
    <button class="btn" onclick={() => (guideOpen = true)}>Guide</button>
    {#if certs.some((c) => c.type === 'ca')}
      <span class="badge badge-success"><span class="status-dot active"></span>CA Ready</span>
    {:else}
      <span class="badge badge-neutral"><span class="status-dot inactive"></span>No CA</span>
    {/if}
    <span class="count-pill">{certs.length} {certs.length === 1 ? 'cert' : 'certs'}</span>
  </div>
</div>

{#if error}
  <div class="error-toast"><span>{error}</span><button onclick={() => (error = '')}>×</button></div>
{/if}

<div class="split section">
  <div class="panel">
    {#if certs.find((c) => c.type === 'ca')}
      {@const caCert = certs.find((c) => c.type === 'ca')}
      <div class="panel-head">
        <h2 class="panel-title">Certificate Authority</h2>
        <span class="badge badge-success">Active</span>
      </div>
      <div class="panel-body">
        <div class="meta-grid">
          <div class="meta-row">
            <div class="meta-key">Name</div>
            <div class="meta-val">{caCert?.name}</div>
          </div>
          <div class="meta-row">
            <div class="meta-key">Type</div>
            <div class="meta-val">Root CA</div>
          </div>
          <div class="meta-row">
            <div class="meta-key">Expires</div>
            <div class="meta-val">{caCert?.expires_at || '-'}</div>
          </div>
        </div>
        <p class="field-desc">
          The CA is initialized. You can now issue server and client certificates.
        </p>
      </div>
    {:else}
      <div class="panel-head">
        <h2 class="panel-title">Certificate Authority</h2>
        <span class="badge badge-warning">Required</span>
      </div>
      <div class="panel-body">
        <p class="section-desc">
          Initialize a Certificate Authority before issuing server or client certificates for
          WPA-Enterprise.
        </p>
        <button class="btn btn-primary" onclick={createCA} disabled={creating}>
          {creating ? 'Initializing…' : 'Initialize CA'}
        </button>
      </div>
    {/if}
  </div>

  <div class="panel">
    <div class="panel-head">
      <h2 class="panel-title">New Certificate</h2>
    </div>
    <div class="panel-body">
      <div class="cert-form">
        <div class="field">
          <label class="field-label" for="newType">Type</label>
          <select class="input" id="newType" bind:value={newType}>
            <option value="server">Server (FreeRADIUS)</option>
            <option value="client">Client (EAP-TLS)</option>
          </select>
        </div>
        {#if newType === 'server'}
          <div class="field">
            <label class="field-label" for="newName">Name</label>
            <input
              class="input"
              id="newName"
              bind:value={newName}
              placeholder="e.g. radius-server"
            />
          </div>
        {:else}
          <div class="field">
            <label class="field-label" for="newUID">User UID</label>
            <input class="input" id="newUID" bind:value={newUID} placeholder="e.g. jdoe" />
          </div>
        {/if}
        <button
          class="btn btn-primary"
          onclick={createCert}
          disabled={creating || (!newName && !newUID)}
        >
          {creating ? 'Creating…' : 'Create Certificate'}
        </button>
      </div>
    </div>
  </div>
</div>

<div class="panel">
  <div class="panel-head">
    <h2 class="panel-title">Certificates</h2>
    {#if !loading && certs.length > 0}
      <span class="count-pill">{certs.length}</span>
    {/if}
  </div>
  {#if loading}
    <div class="empty-state"><p>Loading…</p></div>
  {:else if certs.length === 0}
    <div class="empty-state">
      <strong>No certificates yet</strong>
      <p>Initialize a Certificate Authority to start issuing server and client certificates.</p>
    </div>
  {:else}
    <div class="table-wrap">
      <table class="table">
        <thead>
          <tr><th>Name</th><th>Type</th><th>Network</th><th>Expires</th></tr>
        </thead>
        <tbody>
          {#each certs as c}
            <tr>
              <td data-label="Name" class="mono">{c.name}</td>
              <td data-label="Type"><span class="badge badge-neutral">{c.type}</span></td>
              <td data-label="Network" class="dim">{c.network_id || '-'}</td>
              <td data-label="Expires" class="mono dim">{c.expires_at || '-'}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<GuideModal bind:open={guideOpen} title={GUIDES.certificates.title} doc={GUIDES.certificates.doc} />

<style>
  .section {
    margin-bottom: var(--space-lg);
  }
  .cert-form {
    display: flex;
    flex-direction: column;
    gap: var(--space-md);
  }
  .cert-form .btn {
    align-self: flex-start;
    margin-top: var(--space-xs);
  }
  .field-desc {
    margin-top: var(--space-md);
  }
  .empty-state strong {
    display: block;
    color: var(--text-secondary);
    font-size: var(--font-size-sm);
    font-weight: 600;
  }
</style>
