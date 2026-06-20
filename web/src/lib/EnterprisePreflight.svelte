<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government
  use require a license from VTEM Labs. The Software may not be copied or
  redistributed. See the LICENSE file.
-->
<script lang="ts">
  import { enterprise, type PreflightResult, type ProvisionResult } from '$lib/api';

  // Modal that runs the enterprise readiness check, optionally auto-provisions the
  // missing pieces, then calls onStart() (the parent owns the per-network start call).
  interface Props {
    open: boolean;
    ssid: string;
    onClose: () => void;
    onStart: (autoProvision: boolean) => Promise<void>;
  }
  let { open, ssid, onClose, onStart }: Props = $props();

  let phase = $state<'loading' | 'review' | 'provisioning' | 'starting' | 'done' | 'error'>(
    'loading'
  );
  let preflight = $state<PreflightResult | null>(null);
  let provisionResult = $state<ProvisionResult | null>(null);
  let errorMsg = $state('');

  $effect(() => {
    if (open) {
      void load();
    }
  });

  async function load() {
    phase = 'loading';
    errorMsg = '';
    provisionResult = null;
    try {
      preflight = await enterprise.preflight();
      phase = 'review';
    } catch (e: any) {
      errorMsg = e?.message ?? 'Failed to run preflight';
      phase = 'error';
    }
  }

  async function autoProvisionAndStart() {
    if (!preflight) return;
    phase = 'provisioning';
    errorMsg = '';
    try {
      provisionResult = await enterprise.provision();
      if (!provisionResult.ok) {
        phase = 'error';
        errorMsg = 'Auto-provision reported failures - review the steps below';
        return;
      }
      phase = 'starting';
      await onStart(false); // just provisioned, so preflight passes without auto_provision
      phase = 'done';
    } catch (e: any) {
      phase = 'error';
      errorMsg = e?.message ?? 'Auto-provision failed';
    }
  }

  async function startAsIs() {
    phase = 'starting';
    errorMsg = '';
    try {
      await onStart(false);
      phase = 'done';
    } catch (e: any) {
      phase = 'error';
      errorMsg = e?.message ?? 'Start failed';
    }
  }

  function close() {
    if (phase === 'provisioning' || phase === 'starting') return;
    onClose();
  }
</script>

{#if open}
  <button class="backdrop" onclick={close} aria-label="Close" tabindex="-1"></button>
  <div class="modal" role="dialog" aria-modal="true" aria-labelledby="preflight-title">
    <div class="modal-header">
      <div>
        <h2 id="preflight-title">Enterprise Network Preflight</h2>
        <p class="sub">SSID: <span class="mono">{ssid}</span></p>
      </div>
      <button
        class="close-btn"
        onclick={close}
        disabled={phase === 'provisioning' || phase === 'starting'}>×</button
      >
    </div>

    <div class="modal-body">
      {#if phase === 'loading'}
        <div class="empty-state"><p>Running readiness checks…</p></div>
      {:else if preflight}
        <p class="section-desc" style="margin-bottom: var(--space-md)">
          A WPA-Enterprise SSID needs a CA, a server certificate, an LDAP directory with users, and
          a configured + running FreeRADIUS. The checks below tell you what's missing.
        </p>

        <div class="check-list">
          {#each preflight.checks as c}
            <div class="check-row" class:ok={c.ok}>
              <span class="check-icon">{c.ok ? '✓' : '✗'}</span>
              <div class="check-meta">
                <div class="check-label">{c.label}</div>
                {#if c.detail}<div class="check-detail">{c.detail}</div>{/if}
              </div>
            </div>
          {/each}
        </div>

        {#if provisionResult}
          <div class="section-title" style="margin-top:var(--space-lg)">Auto-Provision Report</div>
          <div class="check-list">
            {#each provisionResult.steps as s}
              <div class="check-row" class:ok={s.status !== 'failed'}>
                <span class="check-icon"
                  >{s.status === 'created' ? '+' : s.status === 'skipped' ? '·' : '✗'}</span
                >
                <div class="check-meta">
                  <div class="check-label">{s.label}<span class="status-tag">{s.status}</span></div>
                  {#if s.detail}<div class="check-detail">{s.detail}</div>{/if}
                </div>
              </div>
            {/each}
          </div>

          {#if provisionResult.users?.length}
            <div class="section-title" style="margin-top:var(--space-lg)">
              Provisioned Credentials
            </div>
            <p class="section-desc">
              Use these to test 802.1X authentication from a client device:
            </p>
            <div class="table-wrap" style="max-height:240px;overflow-y:auto">
              <table class="table">
                <thead><tr><th>UID</th><th>Name</th><th>Password</th></tr></thead>
                <tbody>
                  {#each provisionResult.users as u}
                    <tr>
                      <td class="mono">{u.uid}</td>
                      <td>{u.cn}</td>
                      <td class="mono">{u.password}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {/if}
        {/if}

        {#if errorMsg}
          <div class="error-toast" style="margin-top:var(--space-md)">
            <span>{errorMsg}</span>
          </div>
        {/if}
      {:else if errorMsg}
        <div class="error-toast"><span>{errorMsg}</span></div>
      {/if}
    </div>

    <div class="modal-footer">
      {#if phase === 'review' && preflight}
        {#if preflight.ok}
          <button class="btn" onclick={close}>Cancel</button>
          <button class="btn btn-primary" onclick={startAsIs}>Start Network</button>
        {:else}
          <button class="btn" onclick={close}>Cancel</button>
          <button class="btn btn-primary" onclick={autoProvisionAndStart}>
            Auto-provision &amp; Start
          </button>
        {/if}
      {:else if phase === 'provisioning'}
        <span class="dim">Provisioning dependencies…</span>
      {:else if phase === 'starting'}
        <span class="dim">Starting network…</span>
      {:else if phase === 'done'}
        <button class="btn btn-primary" onclick={close}>Close</button>
      {:else if phase === 'error'}
        <button class="btn" onclick={close}>Close</button>
        <button class="btn btn-primary" onclick={load}>Retry</button>
      {/if}
    </div>
  </div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.65);
    z-index: 500;
    border: none;
    cursor: pointer;
  }
  .modal {
    position: fixed;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    background: var(--bg-secondary);
    border: 1px solid var(--border-primary);
    border-radius: var(--radius-md);
    width: min(640px, 92vw);
    max-height: 88vh;
    display: flex;
    flex-direction: column;
    z-index: 510;
    box-shadow: 0 18px 60px rgba(0, 0, 0, 0.5);
  }
  .modal-header {
    padding: var(--space-lg) var(--space-xl);
    border-bottom: 1px solid var(--border-primary);
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: var(--space-md);
  }
  .modal-header h2 {
    font-size: var(--font-size-lg);
    font-weight: 600;
    color: var(--text-primary);
    margin: 0;
  }
  .sub {
    font-size: var(--font-size-xs);
    color: var(--text-dim);
    margin-top: 2px;
  }
  .close-btn {
    background: none;
    border: none;
    font-size: var(--font-size-xl);
    color: var(--text-dim);
    cursor: pointer;
    line-height: 1;
    padding: 0 var(--space-sm);
  }
  .close-btn:hover:not(:disabled) {
    color: var(--text-primary);
  }
  .close-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
  .modal-body {
    padding: var(--space-lg) var(--space-xl);
    overflow-y: auto;
    flex: 1;
  }
  .modal-footer {
    padding: var(--space-md) var(--space-xl);
    border-top: 1px solid var(--border-primary);
    display: flex;
    gap: var(--space-sm);
    justify-content: flex-end;
    align-items: center;
  }
  .check-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-sm);
  }
  .check-row {
    display: flex;
    gap: var(--space-md);
    padding: var(--space-sm) var(--space-md);
    border: 1px solid var(--border-primary);
    border-radius: var(--radius-sm);
    background: var(--bg-primary);
  }
  .check-row.ok {
    border-color: rgba(34, 197, 94, 0.3);
    background: rgba(34, 197, 94, 0.04);
  }
  .check-icon {
    flex-shrink: 0;
    font-family: var(--font-mono);
    font-size: var(--font-size-md);
    line-height: 1.4;
    width: 20px;
    text-align: center;
    color: var(--status-error);
  }
  .check-row.ok .check-icon {
    color: var(--status-active);
  }
  .check-meta {
    flex: 1;
    min-width: 0;
  }
  .check-label {
    font-size: var(--font-size-sm);
    color: var(--text-primary);
  }
  .check-detail {
    font-size: var(--font-size-xs);
    color: var(--text-dim);
    font-family: var(--font-mono);
    margin-top: 2px;
    word-break: break-all;
  }
  .status-tag {
    display: inline-block;
    margin-left: var(--space-sm);
    font-size: var(--font-size-2xs);
    padding: 1px 6px;
    border-radius: 3px;
    background: var(--bg-hover);
    color: var(--text-dim);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    font-weight: 600;
  }
</style>
