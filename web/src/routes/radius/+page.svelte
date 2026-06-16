<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, paid training, paid CTF,
  or any for-profit use requires a license from VTEM Labs. See the LICENSE file.
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { system, radius } from '$lib/api';
  import { toast } from '$lib/stores/toast';

  let status = $state<'running' | 'stopped' | 'unknown'>('unknown');
  let eapType = $state('peap');
  let innerAuth = $state('mschapv2');
  let sharedSecret = $state('');
  let saving = $state(false);

  onMount(async () => {
    try {
      const data = await system.status();
      status = data.radius_running ? 'running' : 'stopped';
    } catch (e: any) {
      status = 'unknown';
      toast.err(e?.message ?? 'Failed to fetch RADIUS status');
    }
  });

  async function save() {
    saving = true;
    try {
      await radius.saveConfig({
        eap_type: eapType,
        inner_auth: innerAuth,
        shared_secret: sharedSecret
      });
      toast.success('RADIUS configuration saved');
    } catch (e: any) {
      toast.err(e?.message ?? 'Failed to save RADIUS config');
    }
    saving = false;
  }
</script>

<svelte:head><title>RADIUS - Tala WTE</title></svelte:head>

<div class="page-header">
  <div>
    <h1 class="page-title">RADIUS</h1>
    <p class="page-subtitle">FreeRADIUS 3.x - 802.1X enterprise authentication</p>
  </div>
  <div class="header-actions">
    <a href="/radius/guide" class="btn">Guide</a>
    <span class="badge {status === 'running' ? 'badge-success' : status === 'stopped' ? 'badge-error' : 'badge-neutral'}">
      FreeRADIUS {status}
    </span>
  </div>
</div>

<div class="stack">
  <div class="panel">
    <div class="stat-strip">
      <div class="stat-cell">
        <span class="k">RADIUS Port</span>
        <span class="v">1812</span>
        <span class="cell-sub">Authentication</span>
      </div>
      <div class="stat-cell">
        <span class="k">Accounting Port</span>
        <span class="v">1813</span>
        <span class="cell-sub">Accounting</span>
      </div>
      <div class="stat-cell">
        <span class="k">LDAP Backend</span>
        <span class="v ldap-v">127.0.0.1:3389</span>
        <span class="cell-sub">OpenLDAP (slapd)</span>
      </div>
    </div>
  </div>

  <div class="split">
    <div class="panel">
      <div class="panel-head">
        <h2 class="panel-title">EAP Configuration</h2>
      </div>
      <div class="panel-body">
        <div class="field">
          <label class="field-label" for="eapType">Default EAP Type</label>
          <select class="input" id="eapType" bind:value={eapType}>
            <option value="peap">EAP-PEAP (Recommended)</option>
            <option value="tls">EAP-TLS (Certificate Auth)</option>
            <option value="ttls">EAP-TTLS</option>
            <option value="fast">EAP-FAST</option>
          </select>
        </div>
        <div class="field">
          <label class="field-label" for="innerAuth">Inner Authentication</label>
          <select class="input" id="innerAuth" bind:value={innerAuth}>
            <option value="mschapv2">MSCHAPv2</option>
            <option value="pap">PAP</option>
            <option value="chap">CHAP</option>
            <option value="gtc">GTC</option>
          </select>
        </div>
        <div class="field">
          <label class="field-label" for="sharedSecret">Shared Secret</label>
          <input class="input" id="sharedSecret" type="password" bind:value={sharedSecret} placeholder="Leave blank to use generated secret" />
        </div>
        <button class="btn btn-primary save-btn" onclick={save} disabled={saving}>
          {saving ? 'Saving...' : 'Save Configuration'}
        </button>
      </div>
    </div>

    <div class="panel">
      <div class="panel-head">
        <h2 class="panel-title">Authentication Chain</h2>
      </div>
      <div class="panel-body">
        <div class="chain">
          <div class="chain-node">Wi-Fi Client</div>
          <div class="chain-arrow">↓ &nbsp;EAP-{eapType.toUpperCase()}{eapType === 'tls' ? ' (client certificate)' : ' / ' + innerAuth.toUpperCase()}</div>
          <div class="chain-node">hostapd (AP)</div>
          <div class="chain-arrow">↓ &nbsp;RADIUS Access-Request</div>
          <div class="chain-node">FreeRADIUS :1812</div>
          <div class="chain-arrow">↓ &nbsp;LDAP bind</div>
          <div class="chain-node">OpenLDAP :3389</div>
          <div class="chain-arrow">↓ &nbsp;Access-Accept + MSK</div>
          <div class="chain-node chain-final">4-Way Handshake → Connected</div>
        </div>
        <div class="chain-links">
          <a href="/ldap" class="btn btn-sm">Manage LDAP Users →</a>
          <a href="/certificates" class="btn btn-sm">Manage Certs →</a>
        </div>
      </div>
    </div>
  </div>
</div>

<style>
  .cell-sub {
    font-size: var(--font-size-xs);
    color: var(--text-dim);
  }
  .ldap-v {
    font-size: var(--font-size-md);
    font-family: var(--font-mono);
  }

  .panel-body .field + .field { margin-top: var(--space-lg); }
  .save-btn { margin-top: var(--space-xl); }

  .chain {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    line-height: 1.5;
    background: var(--bg-input);
    border: 1px solid var(--border-primary);
    border-radius: var(--radius-md);
    padding: var(--space-lg);
  }
  .chain-node { color: var(--text-primary); font-weight: 600; }
  .chain-final { color: var(--accent-hover); }
  .chain-arrow { color: var(--text-dim); padding: var(--space-xs) 0 var(--space-xs) var(--space-md); }

  .chain-links {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-sm);
    margin-top: var(--space-lg);
  }
</style>
