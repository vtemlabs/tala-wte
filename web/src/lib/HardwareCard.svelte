<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government
  use require a license from VTEM Labs. The Software may not be copied or
  redistributed. See the LICENSE file.
-->
<script lang="ts">
  // The adapter carries the full backend iface.Adapter shape (more fields than
  // the WirelessInterface summary type); typed loosely until a shared type exists.
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let { adapter }: { adapter: any } = $props();

  const model = $derived(
    adapter.manufacturer && adapter.device_model
      ? `${adapter.manufacturer} ${adapter.device_model}`
      : adapter.driver || 'Unknown device'
  );
  const txMax = $derived(
    adapter.tx_power_max > 0 ? `${adapter.tx_power_max / 100} dBm` : 'Driver-managed'
  );
  // Set when this adapter is claimed by a running network (its phy is in that
  // network's namespace). Full hardware detail is cached, so the card stays rich.
  const inUseBy = $derived(adapter.in_use_by || '');
</script>

<div class="hw-card">
  <div class="hw-head">
    <span class="status-dot active"></span>
    <div class="hw-id">
      <span class="hw-iface">{adapter.interface}</span>
      <span class="hw-model">{model}</span>
    </div>
    {#if inUseBy}<span class="badge badge-success" title="In use by {inUseBy}">in use: {inUseBy}</span
      >{/if}
    {#if adapter.chipset}<span class="hw-chip">{adapter.chipset}</span>{/if}
  </div>

  <div class="hw-specs">
    <div class="hw-spec"><span>USB ID</span><b class="mono">{adapter.usb_id || 'system'}</b></div>
    <div class="hw-spec"><span>MAC</span><b class="mono">{adapter.mac_address || '-'}</b></div>
    {#if adapter.standard}<div class="hw-spec">
        <span>Standard</span><b>{adapter.standard}</b>
      </div>{/if}
    <div class="hw-spec"><span>TX Power</span><b>{txMax}</b></div>
    {#if adapter.max_channel_width > 0}<div class="hw-spec">
        <span>Max Width</span><b>{adapter.max_channel_width} MHz</b>
      </div>{/if}
    {#if adapter.stock_antenna_count > 0}<div class="hw-spec">
        <span>Antennas</span><b
          >{adapter.stock_antenna_count}x {adapter.antenna_connector || ''}
          {adapter.stock_antenna_gain_dbi || 0}dBi</b
        >
      </div>{/if}
  </div>

  <div class="hw-tags">
    {#if adapter.tx_power_adjustable}<span class="hw-tag cap">TX adj</span>{/if}
    {#if adapter.has_dfs}<span class="hw-tag cap">DFS</span>{/if}
    {#if adapter.monitor_bands?.length}<span class="hw-tag cap">monitor</span>{/if}
    {#if adapter.injection_bands?.length}<span class="hw-tag cap">inject</span>{/if}
    {#each adapter.bands || [] as band}<span class="hw-tag">{band}</span>{/each}
  </div>

  {#if adapter.limits?.length}
    <div class="hw-limits">
      <span class="hw-limits-title">Limits</span>
      {#each adapter.limits as limit}<span class="hw-limit">{limit}</span>{/each}
    </div>
  {/if}

  {#if adapter.notes}
    <details class="hw-notes">
      <summary>Lab notes</summary>
      <p>{adapter.notes}</p>
    </details>
  {/if}
</div>

<style>
  .hw-card {
    background: var(--bg-tertiary);
    border: 1px solid var(--border-primary);
    border-radius: var(--radius-md);
    padding: var(--space-md) var(--space-lg);
    display: flex;
    flex-direction: column;
    gap: var(--space-md);
    transition: border-color var(--transition-fast);
  }
  .hw-card:hover {
    border-color: var(--border-secondary);
  }

  .hw-head {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
  }
  .hw-head .status-dot {
    margin: 0;
    flex-shrink: 0;
  }
  .hw-id {
    display: flex;
    flex-direction: column;
    min-width: 0;
    flex: 1;
  }
  .hw-iface {
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    font-weight: 700;
    color: var(--text-primary);
  }
  .hw-model {
    font-size: var(--font-size-xs);
    color: var(--text-muted);
  }
  .hw-chip {
    flex-shrink: 0;
    font-family: var(--font-mono);
    font-size: 10px;
    font-weight: 600;
    color: var(--accent-hover);
    background: var(--accent-soft);
    border: 1px solid rgba(47, 129, 247, 0.3);
    border-radius: 999px;
    padding: 2px 9px;
  }
  .hw-head .badge {
    flex-shrink: 0;
    max-width: 200px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .hw-specs {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-sm) var(--space-lg);
  }
  .hw-spec {
    display: flex;
    flex-direction: column;
    gap: 1px;
    min-width: 0;
  }
  .hw-spec span {
    font-size: 10px;
    font-weight: 600;
    color: var(--text-dim);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .hw-spec b {
    font-size: var(--font-size-xs);
    font-weight: 500;
    color: var(--text-primary);
    word-break: break-all;
  }

  .hw-tags {
    display: flex;
    flex-wrap: wrap;
    gap: 5px;
  }
  .hw-tag {
    font-family: var(--font-mono);
    font-size: 10px;
    font-weight: 600;
    background: var(--bg-elevated);
    border: 1px solid var(--border-primary);
    color: var(--text-muted);
    border-radius: var(--radius-sm);
    padding: 2px 7px;
  }
  .hw-tag.cap {
    color: var(--color-cyan);
    border-color: rgba(34, 211, 238, 0.3);
    background: rgba(34, 211, 238, 0.08);
  }

  .hw-limits {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 6px;
    margin-top: var(--space-sm);
    padding: var(--space-sm) var(--space-md);
    border: 1px solid var(--color-yellow);
    border-radius: var(--radius-sm);
  }
  .hw-limits-title {
    font-size: var(--font-size-xs);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--color-yellow);
  }
  .hw-limit {
    font-size: var(--font-size-xs);
    color: var(--text-secondary);
  }
  .hw-limit:not(:last-child)::after {
    content: ',';
    color: var(--text-muted);
  }

  .hw-notes {
    font-size: var(--font-size-xs);
    color: var(--text-muted);
    line-height: 1.5;
  }
  .hw-notes summary {
    cursor: pointer;
    color: var(--text-dim);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    font-size: 10px;
  }
  .hw-notes p {
    margin-top: var(--space-sm);
  }
</style>
