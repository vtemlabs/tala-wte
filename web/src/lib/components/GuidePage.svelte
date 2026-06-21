<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government
  use require a license from VTEM Labs. The Software may not be copied or
  redistributed. See the LICENSE file.
-->
<script lang="ts">
  // Reusable in-app documentation page: back link, title, and a Markdown body.
  import { mdToHtml } from '$lib/md';
  import { lightbox } from '$lib/lightbox';
  import { docScale, zoomDoc } from '$lib/stores/docScale';

  let {
    title,
    subtitle = '',
    backHref,
    backLabel = 'Back',
    crumb = '',
    doc
  }: {
    title: string;
    subtitle?: string;
    backHref: string;
    backLabel?: string;
    crumb?: string;
    doc: string;
  } = $props();
</script>

<svelte:head><title>{title} - Tala WTE</title></svelte:head>

<div class="page-header">
  <div>
    {#if crumb}<a class="crumb" href={backHref}>{crumb}</a>{/if}
    <h1 class="page-title">{title}</h1>
    {#if subtitle}<p class="page-subtitle">{subtitle}</p>{/if}
  </div>
  <div class="header-actions">
    <div class="doc-font">
      <button
        class="btn doc-font-btn"
        onclick={() => zoomDoc(-0.1)}
        title="Smaller text"
        aria-label="Smaller text">A-</button
      >
      <span class="doc-fs">{Math.round($docScale * 100)}%</span>
      <button
        class="btn doc-font-btn"
        onclick={() => zoomDoc(0.1)}
        title="Larger text"
        aria-label="Larger text">A+</button
      >
    </div>
    <a href={backHref} class="btn">{backLabel}</a>
  </div>
</div>

<article class="panel prose-panel">
  <div
    class="tala-doc"
    use:lightbox
    style="font-size: calc(var(--font-size-sm) * 1.1 * {$docScale})"
  >
    <!-- eslint-disable-next-line svelte/no-at-html-tags -->
    {@html mdToHtml(doc)}
  </div>
</article>

<style>
  .crumb {
    font-size: var(--font-size-xs);
    color: var(--text-dim);
  }
  .crumb:hover {
    color: var(--accent-hover);
  }

  .prose-panel {
    padding: var(--space-2xl) var(--space-3xl);
  }
  /* Document body styling is global (.tala-doc in app.css), shared with the Guide window. */
  .doc-font {
    display: flex;
    align-items: center;
    gap: var(--space-xs);
  }
  .doc-font-btn {
    padding: var(--space-xs) var(--space-sm);
    font-weight: 600;
    min-width: 34px;
  }
  .doc-fs {
    font-size: var(--font-size-xs);
    color: var(--text-dim);
    min-width: 40px;
    text-align: center;
    font-variant-numeric: tabular-nums;
  }

  @media (max-width: 700px) {
    .prose-panel {
      padding: var(--space-xl) var(--space-lg);
    }
  }
</style>
