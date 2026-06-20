<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government use
  require a license from VTEM Labs. See the LICENSE file.
-->
<footer class="app-footer">
  <img class="footer-logo" src="/brand/vtem-labs.png" alt="VTEM Labs" />
  <span class="footer-copy">(c) 2026 VTEM Labs, Inc. All rights reserved.</span>
  <span class="footer-sep">|</span>
  <a class="footer-link" href="https://vtemlabs.com" target="_blank" rel="noopener noreferrer"
    >vtemlabs.com</a
  >
</footer>

<style>
  .app-footer {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-md);
    flex-wrap: wrap;
    padding: var(--space-lg) var(--space-xl);
    /* Top margin gaps it off the page content (.page-area pins it to the bottom);
       the negative left/right/bottom cancel the .content padding so it spans the
       full width and sits flush against the bottom edge. */
    margin: var(--space-2xl) calc(-1 * var(--space-2xl)) calc(-1 * var(--space-2xl));
    border-top: 1px solid var(--border-primary);
    color: var(--text-dim);
    font-size: var(--font-size-xs);
  }
  .footer-logo {
    height: 15px;
    width: auto;
    opacity: 0.85;
  }
  .footer-copy {
    color: var(--text-dim);
  }
  .footer-sep {
    color: var(--border-secondary);
  }
  .footer-link {
    color: var(--text-muted);
    text-decoration: none;
    transition: color var(--transition-fast);
  }
  .footer-link:hover {
    color: var(--accent-hover);
    text-decoration: underline;
  }
  @media (max-width: 900px) {
    .app-footer {
      /* On mobile .content padding is space-lg, so match the negative margin to
         it; the desktop -space-2xl overshoots by 16px and overflows the page.
         Stack the parts so the long copyright line stops wrapping into the logo. */
      flex-direction: column;
      gap: var(--space-xs);
      text-align: center;
      margin-right: calc(-1 * var(--space-lg));
      margin-left: calc(-1 * var(--space-lg));
      margin-bottom: calc(-1 * var(--space-lg));
      padding: var(--space-lg);
    }
    .footer-sep {
      display: none;
    }
  }
</style>
