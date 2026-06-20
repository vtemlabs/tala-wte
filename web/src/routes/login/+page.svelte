<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government
  use require a license from VTEM Labs. The Software may not be copied or
  redistributed. See the LICENSE file.
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { pb } from '$lib/api';
  import LicenseModal from '$lib/components/LicenseModal.svelte';

  // Admin is created here in the browser only; never auto-provisioned or printed.
  let mode = $state<'loading' | 'setup' | 'login'>('loading');
  let email = $state('');
  let password = $state('');
  let confirm = $state('');
  let setupToken = $state('');
  let error = $state('');
  let loading = $state(false);

  // License acknowledgment is mandatory before the first admin account can be created.
  let licenseAck = $state(false);
  let showLicense = $state(false);

  onMount(async () => {
    try {
      const res = await fetch('/api/wte/setup/status');
      const data = await res.json();
      mode = data?.needs_setup ? 'setup' : 'login';
    } catch {
      mode = 'login';
    }
  });

  async function login() {
    loading = true;
    error = '';
    try {
      // PocketBase v0.23+ superuser auth targets the _superusers collection.
      await pb.collection('_superusers').authWithPassword(email, password);
      goto('/');
    } catch {
      error = 'Invalid credentials. Please try again.';
    }
    loading = false;
  }

  async function createAccount() {
    error = '';
    if (password.length < 10) {
      error = 'Password must be at least 10 characters.';
      return;
    }
    if (password !== confirm) {
      error = 'Passwords do not match.';
      return;
    }
    if (!licenseAck) {
      error = 'You must acknowledge the license to continue.';
      return;
    }
    loading = true;
    try {
      const res = await fetch('/api/wte/setup/complete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password, setup_token: setupToken, license_ack: licenseAck })
      });
      const data = await res.json();
      if (!res.ok) {
        error = data?.error || 'Could not create the admin account.';
        loading = false;
        return;
      }
      pb.authStore.save(data.token, data.record);
      goto('/');
    } catch {
      error = 'Could not reach the server. Please try again.';
      loading = false;
    }
  }
</script>

<svelte:head><title>{mode === 'setup' ? 'Set up' : 'Login'} - Tala WTE</title></svelte:head>

<div class="login-page">
  <section class="login-left">
    <div class="login-col">
      <div class="brand-lockup">
        <img class="brand-logo" src="/brand/tala-logo.png" alt="Tala" />
        <span class="brand-div"></span>
        <span class="brand-wte">WTE</span>
      </div>

      <div class="card-intro">
        <h1 class="card-heading">
          {mode === 'setup' ? 'Create administrator' : 'Sign in'}
        </h1>
        <p class="card-sub">
          {#if mode === 'setup'}
            First run. Provision the admin account for this instance.
          {:else if mode === 'login'}
            Authenticate to access the training environment.
          {:else}
            Establishing session.
          {/if}
        </p>
      </div>

      {#if error}
        <div class="error-toast">
          <span>{error}</span>
          <button class="error-dismiss" onclick={() => (error = '')} aria-label="Dismiss">×</button>
        </div>
      {/if}

      {#if mode === 'loading'}
        <div class="login-loading">
          <span class="status-dot inactive"></span>
          <span>Loading…</span>
        </div>
      {:else if mode === 'setup'}
        <form
          class="login-form"
          onsubmit={(e) => {
            e.preventDefault();
            createAccount();
          }}
        >
          <div class="form-group">
            <label class="field-label" for="setup-token">Setup Token</label>
            <input
              class="input"
              type="text"
              id="setup-token"
              bind:value={setupToken}
              placeholder="from the server log"
              required
              autocomplete="off"
            />
            <span class="field-desc"
              >Shown in the server log at first boot (run: journalctl -u tala-wte, line "SETUP
              TOKEN").</span
            >
          </div>
          <div class="form-group">
            <label class="field-label" for="email">Admin Email</label>
            <input
              class="input"
              type="email"
              id="email"
              bind:value={email}
              placeholder="admin@tala.wte"
              required
              autocomplete="username"
            />
          </div>
          <div class="form-group">
            <label class="field-label" for="password">Password</label>
            <input
              class="input"
              type="password"
              id="password"
              bind:value={password}
              placeholder="At least 10 characters"
              required
              autocomplete="new-password"
            />
            <span class="field-desc">Minimum 10 characters. Stored hashed; never displayed.</span>
          </div>
          <div class="form-group">
            <label class="field-label" for="confirm">Confirm Password</label>
            <input
              class="input"
              type="password"
              id="confirm"
              bind:value={confirm}
              placeholder="Re-enter password"
              required
              autocomplete="new-password"
            />
          </div>
          <label class="ack">
            <input type="checkbox" bind:checked={licenseAck} />
            <span
              >I have read and agree to the <button
                type="button"
                class="ack-link"
                onclick={() => (showLicense = true)}>Tala WTE License</button
              >.</span
            >
          </label>
          <button
            type="submit"
            class="btn btn-primary login-submit"
            disabled={loading || !licenseAck}
          >
            {loading ? 'Creating…' : 'Create Admin Account'}
          </button>
        </form>
      {:else}
        <form
          class="login-form"
          onsubmit={(e) => {
            e.preventDefault();
            login();
          }}
        >
          <div class="form-group">
            <label class="field-label" for="email">Admin Email</label>
            <input
              class="input"
              type="email"
              id="email"
              bind:value={email}
              placeholder="admin@tala.wte"
              required
              autocomplete="username"
            />
          </div>
          <div class="form-group">
            <label class="field-label" for="password">Password</label>
            <input
              class="input"
              type="password"
              id="password"
              bind:value={password}
              placeholder="••••••••"
              required
              autocomplete="current-password"
            />
          </div>
          <button type="submit" class="btn btn-primary login-submit" disabled={loading}>
            {loading ? 'Signing in…' : 'Sign In'}
          </button>
        </form>
      {/if}

      <p class="login-foot">Authorized training use only. Sessions are scoped to this instance.</p>
    </div>
  </section>

  <aside class="login-right" aria-hidden="true">
    <div class="brand-stage">
      <img class="brand-wolf" src="/brand/tala-wolf.png" alt="" />
      <div class="brand-tagline">Wireless Training Environment</div>
    </div>
  </aside>
</div>

<LicenseModal bind:open={showLicense} />

<style>
  .login-page {
    display: grid;
    grid-template-columns: 1.05fr 0.95fr;
    min-height: 100vh;
  }

  .login-left {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-2xl);
    background: var(--bg-primary);
  }
  .login-col {
    width: 100%;
    max-width: 384px;
    display: flex;
    flex-direction: column;
  }

  .brand-lockup {
    display: flex;
    align-items: center;
    gap: 13px;
    margin-bottom: var(--space-3xl);
  }
  .brand-logo {
    height: 30px;
    width: auto;
    display: block;
  }
  .brand-div {
    width: 1px;
    height: 22px;
    background: var(--border-secondary);
  }
  .brand-wte {
    font-size: var(--font-size-sm);
    font-weight: 800;
    letter-spacing: 0.18em;
    color: var(--accent-hover);
  }

  .card-intro {
    margin-bottom: var(--space-xl);
  }
  .card-heading {
    font-size: var(--font-size-xl);
    font-weight: 700;
    color: var(--text-primary);
    letter-spacing: -0.01em;
  }
  .card-sub {
    font-size: var(--font-size-sm);
    color: var(--text-muted);
    margin-top: var(--space-xs);
  }

  .login-form {
    display: flex;
    flex-direction: column;
    gap: var(--space-lg);
  }
  .form-group {
    display: flex;
    flex-direction: column;
    gap: var(--space-xs);
  }

  .login-submit {
    width: 100%;
    justify-content: center;
    margin-top: var(--space-sm);
    padding: 11px var(--space-lg);
    font-size: var(--font-size-base);
    font-weight: 600;
  }
  .login-submit:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .ack {
    display: flex;
    align-items: flex-start;
    gap: 9px;
    font-size: var(--font-size-sm);
    color: var(--text-muted);
    line-height: 1.5;
    cursor: pointer;
  }
  .ack input[type='checkbox'] {
    margin-top: 2px;
    width: 15px;
    height: 15px;
    accent-color: var(--accent);
    flex-shrink: 0;
    cursor: pointer;
  }
  .ack-link {
    background: none;
    border: none;
    padding: 0;
    color: var(--accent-hover);
    font: inherit;
    cursor: pointer;
    text-decoration: underline;
  }
  .ack-link:hover {
    color: var(--accent);
  }

  .login-loading {
    display: flex;
    align-items: center;
    gap: var(--space-sm);
    padding: var(--space-xl) 0;
    font-size: var(--font-size-sm);
    color: var(--text-muted);
  }

  .error-toast {
    margin-bottom: var(--space-lg);
  }
  .error-dismiss {
    background: none;
    border: none;
    color: inherit;
    cursor: pointer;
    font-size: var(--font-size-lg);
    line-height: 1;
    padding: 0 var(--space-xs);
    opacity: 0.7;
    transition: opacity var(--transition-fast);
  }
  .error-dismiss:hover {
    opacity: 1;
  }

  .login-foot {
    margin-top: var(--space-3xl);
    font-size: var(--font-size-xs);
    color: var(--text-dim);
  }

  .login-right {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    overflow: hidden;
    border-left: 1px solid var(--border-primary);
    background:
      radial-gradient(circle at 50% 42%, var(--accent-soft), transparent 62%), var(--bg-secondary);
  }
  .login-right::before {
    content: '';
    position: absolute;
    inset: 0;
    background-image:
      linear-gradient(var(--border-subtle) 1px, transparent 1px),
      linear-gradient(90deg, var(--border-subtle) 1px, transparent 1px);
    background-size: 46px 46px;
    opacity: 0.4;
    -webkit-mask-image: radial-gradient(circle at 50% 44%, #000 28%, transparent 74%);
    mask-image: radial-gradient(circle at 50% 44%, #000 28%, transparent 74%);
  }
  .brand-stage {
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-2xl);
  }
  .brand-wolf {
    width: min(44vh, 348px);
    height: auto;
    display: block;
    filter: drop-shadow(0 22px 60px rgba(0, 0, 0, 0.6));
  }
  .brand-tagline {
    font-size: var(--font-size-sm);
    font-weight: 600;
    letter-spacing: 0.34em;
    text-transform: uppercase;
    color: var(--text-muted);
  }

  @media (max-width: 900px) {
    .login-page {
      grid-template-columns: 1fr;
    }
    .login-right {
      display: none;
    }
    .login-left {
      padding: var(--space-2xl) var(--space-xl);
    }
  }
</style>
