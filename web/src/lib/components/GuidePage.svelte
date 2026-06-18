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
    <a href={backHref} class="btn">{backLabel}</a>
  </div>
</div>

<article class="panel prose-panel">
  <div class="prose">
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
  /* No max-width so the guide fills the frame instead of a narrow left column. */
  .prose {
    max-width: none;
    width: 100%;
    color: var(--text-secondary);
  }
  .prose :global(h2) {
    font-size: var(--font-size-lg);
    font-weight: 700;
    color: var(--text-primary);
    margin: var(--space-2xl) 0 var(--space-sm);
    padding-top: var(--space-lg);
    border-top: 1px solid var(--border-subtle);
    letter-spacing: -0.01em;
  }
  .prose :global(h2:first-child) {
    margin-top: 0;
    padding-top: 0;
    border-top: none;
  }
  .prose :global(h3) {
    font-size: var(--font-size-base);
    font-weight: 700;
    color: var(--text-primary);
    margin: var(--space-xl) 0 var(--space-xs);
  }
  .prose :global(p) {
    font-size: var(--font-size-sm);
    line-height: 1.7;
    margin: 0 0 var(--space-md);
  }
  .prose :global(ul),
  .prose :global(ol) {
    margin: 0 0 var(--space-md);
    padding-left: var(--space-xl);
  }
  .prose :global(li) {
    font-size: var(--font-size-sm);
    line-height: 1.7;
    margin-bottom: var(--space-xs);
  }
  .prose :global(strong) {
    color: var(--text-primary);
    font-weight: 700;
  }
  .prose :global(a) {
    color: var(--accent-hover);
  }
  .prose :global(a:hover) {
    text-decoration: underline;
  }
  .prose :global(code) {
    font-family: var(--font-mono);
    font-size: 0.88em;
    color: var(--accent-hover);
    background: var(--bg-input);
    border: 1px solid var(--border-subtle);
    padding: 1px 6px;
    border-radius: var(--radius-sm);
  }
  .prose :global(img) {
    display: block;
    width: 100%;
    max-width: 100%;
    margin: var(--space-lg) 0;
    border: 1px solid var(--border-primary);
    border-radius: var(--radius-md);
    box-shadow: var(--shadow-md);
  }
  .prose :global(blockquote) {
    margin: 0 0 var(--space-md);
    padding: var(--space-sm) var(--space-lg);
    border-left: 3px solid var(--accent);
    background: var(--accent-soft);
    border-radius: var(--radius-sm);
    color: var(--text-secondary);
    font-size: var(--font-size-sm);
  }
  .prose :global(hr) {
    border: none;
    border-top: 1px solid var(--border-subtle);
    margin: var(--space-xl) 0;
  }

  @media (max-width: 700px) {
    .prose-panel {
      padding: var(--space-xl) var(--space-lg);
    }
  }
</style>
