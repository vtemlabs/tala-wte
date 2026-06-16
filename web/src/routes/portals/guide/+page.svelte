<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, paid training, paid CTF,
  or any for-profit use requires a license from VTEM Labs. See the LICENSE file.
-->
<script lang="ts">
  import { mdToHtml } from '$lib/md';

  const EXAMPLES = [
    { img: '/guide/portal-coffee-driftwood.png', name: 'Driftwood Coffee Co.', cat: 'Coffee & Retail' },
    { img: '/guide/portal-hotel-aria.png', name: 'Aria Hotels', cat: 'Hotel' },
    { img: '/guide/portal-airport-summit.png', name: 'Summit International Airport', cat: 'Airport' },
    { img: '/guide/portal-corp-meraki.png', name: 'Continuum Logistics Guest', cat: 'Corporate Guest' }
  ];

  const DOC = `## Overview

A **captive portal** is the splash page a client sees after joining an open Wi-Fi network, before it can reach the internet. In Tala WTE a portal is assigned to an **Open** network; when a client connects, all of its web traffic is intercepted and redirected to the portal until it submits the form. Every value the client enters is captured to **Captured Data**, and portals can optionally validate those credentials against the embedded directory, behaving like a real credentialed hotspot.

Use portals to demonstrate how convincingly a rogue access point harvests credentials and personal information from a fake login page.

## The portal library

The **Captive Portals** page is your template library. Each card is a portal you can preview, edit, clone, or assign to a network.

![The portal library](/guide/gallery.png)

- **Built-in** templates ship with Tala WTE and model realistic venues: coffee shops, hotels, corporate guest pages, airports, in-flight, and ISP hotspots. They are managed by the app and kept current automatically.
- **Custom** templates are ones you create, upload, or clone. Editing a built-in clones it first, so the originals stay intact.
- Use the **category chips** to filter the library, and the **Captured Data** link at the top to jump to everything harvested so far.

## Creating a portal

There are four ways to add a portal. All of them land you in the editor, where you can refine the HTML and preview it live.

### 1. Clone a built-in template

The fastest start. On any built-in card, click **Clone**. You get an editable copy named "<template> (copy)" that you can rename and customize without touching the original.

### 2. Start from scratch

Click **+ New Portal**. You begin with a blank editor, or a starter skeleton via **Insert Starter HTML**. Paste or write any HTML; Tala WTE auto-wires the first login form to capture and (optionally) authenticate, so you do not have to hand-edit form actions.

### 3. Clone from a live URL

Click **Clone from URL** and paste the address of a real sign-in page, for example a vendor hotspot login. Tala WTE fetches the page, inlines its assets, and imports it as a portal. Review the result in the editor before using it.

### 4. Upload a template

Click **Upload Template** to import an \`.html\` file or a \`.zip\` bundle (HTML plus its images, CSS, and JS). Bundles are served as a small static site, so multi-file portals keep their assets and links.

## Editing a portal

Opening a portal shows the editor: the **HTML Source** on the left and a **Live Preview** on the right that updates as you type. Edit the markup, then click **Save**.

![The portal editor](/guide/editor.png)

You do not need to wire forms by hand. On save and on serve, Tala WTE normalizes the portal: it points the first form at the capture endpoint, tags recognized username and password fields, and adds a redirect so the client lands on a normal page after submitting. Field names like \`username\`, \`email\`, \`member_id\`, \`password\`, and \`pin\` are detected automatically.

## Capturing credentials and PII

Every field a client submits is recorded to **Captured Data** with the client's MAC address, IP, and browser. This is the payoff of the exercise.

![Captured data](/guide/captured.png)

When a portal is assigned to a network with **Validate credentials** enabled, Tala WTE checks the submitted username and password against the embedded LDAP directory before granting access, exactly like a real credentialed hotspot. The result (\`success\` or \`fail\`) is stored alongside each submission, and failed logins are re-prompted instead of being waved through.

## Assigning a portal to a network

A portal does nothing until it is attached to a running **Open** network.

1. Create or edit a network and set the **Security Protocol** to **Open**.
2. Enable the captive portal and choose a template from the list.
3. Optionally turn on **Validate credentials** to authenticate against the directory.
4. Start the network. Connecting clients are redirected to your portal until they submit the form.

## Legal links work

The built-in templates link to **Terms of Service**, **Acceptable Use Policy**, and **Privacy Policy**. These resolve to real generic policy pages served by the portal, so the splash feels complete and legitimate to a connecting client. Each page links back to the sign-in screen.

## Tips

- Preview before you deploy: use the **Preview** action on any card, or the live preview in the editor.
- Keep credential field names conventional so auto-capture and validation work without manual wiring.
- Clone, do not edit, when you want a variation of a built-in template.
- Match the venue. The closer your portal looks to a network the client expects, the more convincing the exercise.`;
</script>

<svelte:head><title>Portal Guide - Tala WTE</title></svelte:head>

<div class="page-header">
  <div>
    <a class="crumb" href="/portals">Captive Portals</a>
    <h1 class="page-title">Portal Guide</h1>
    <p class="page-subtitle">How to create, edit, clone, and deploy captive portals</p>
  </div>
  <div class="header-actions">
    <a href="/portals" class="btn">Back to Portals</a>
  </div>
</div>

<section class="examples">
  {#each EXAMPLES as ex}
    <figure class="ex-card">
      <div class="ex-shot"><img src={ex.img} alt={ex.name} loading="lazy" /></div>
      <figcaption>
        <span class="ex-name">{ex.name}</span>
        <span class="ex-cat">{ex.cat}</span>
      </figcaption>
    </figure>
  {/each}
</section>

<article class="panel prose-panel">
  <div class="prose">
    <!-- eslint-disable-next-line svelte/no-at-html-tags -->
    {@html mdToHtml(DOC)}
  </div>
</article>

<style>
  .crumb { font-size: var(--font-size-xs); color: var(--text-dim); }
  .crumb:hover { color: var(--accent-hover); }

  .examples {
    display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
    gap: var(--space-lg); margin-bottom: var(--space-xl);
  }
  .ex-card {
    margin: 0; background: var(--bg-card); border: 1px solid var(--border-primary);
    border-radius: var(--radius-lg); overflow: hidden; box-shadow: var(--shadow-md);
  }
  .ex-shot { background: var(--bg-secondary); padding: var(--space-md); display: flex; justify-content: center; }
  .ex-shot img { width: 100%; max-width: 220px; border-radius: var(--radius-sm); display: block; }
  .ex-card figcaption {
    display: flex; flex-direction: column; gap: 2px;
    padding: var(--space-md) var(--space-lg); border-top: 1px solid var(--border-primary);
  }
  .ex-name { font-size: var(--font-size-sm); font-weight: 600; color: var(--text-primary); }
  .ex-cat { font-size: 11px; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.05em; }

  .prose-panel { padding: var(--space-2xl) var(--space-3xl); }
  .prose { max-width: none; width: 100%; color: var(--text-secondary); }
  .prose :global(h2) {
    font-size: var(--font-size-lg); font-weight: 700; color: var(--text-primary);
    margin: var(--space-2xl) 0 var(--space-sm); padding-top: var(--space-lg);
    border-top: 1px solid var(--border-subtle); letter-spacing: -0.01em;
  }
  .prose :global(h2:first-child) { margin-top: 0; padding-top: 0; border-top: none; }
  .prose :global(h3) {
    font-size: var(--font-size-base); font-weight: 700; color: var(--text-primary);
    margin: var(--space-xl) 0 var(--space-xs);
  }
  .prose :global(p) { font-size: var(--font-size-sm); line-height: 1.7; margin: 0 0 var(--space-md); }
  .prose :global(ul), .prose :global(ol) { margin: 0 0 var(--space-md); padding-left: var(--space-xl); }
  .prose :global(li) { font-size: var(--font-size-sm); line-height: 1.7; margin-bottom: var(--space-xs); }
  .prose :global(strong) { color: var(--text-primary); font-weight: 700; }
  .prose :global(a) { color: var(--accent-hover); }
  .prose :global(a:hover) { text-decoration: underline; }
  .prose :global(code) {
    font-family: var(--font-mono); font-size: 0.88em; color: var(--accent-hover);
    background: var(--bg-input); border: 1px solid var(--border-subtle);
    padding: 1px 6px; border-radius: var(--radius-sm);
  }
  .prose :global(img) {
    display: block; width: 100%; max-width: 100%; margin: var(--space-lg) 0;
    border: 1px solid var(--border-primary); border-radius: var(--radius-md);
    box-shadow: var(--shadow-md);
  }
  .prose :global(hr) { border: none; border-top: 1px solid var(--border-subtle); margin: var(--space-xl) 0; }

  @media (max-width: 700px) {
    .prose-panel { padding: var(--space-xl) var(--space-lg); }
  }
</style>
