<!--
  Tala WTE - Wireless Training Environment
  Copyright (c) 2026 VTEM Labs. All rights reserved.
  Free for personal and non-profit use. Commercial, for-profit, and government
  use require a license from VTEM Labs. See the LICENSE file.
-->
<script lang="ts">
  import GuidePage from '$lib/components/GuidePage.svelte';
  import { GUIDES } from '$lib/guides';
</script>

<GuidePage
  title={GUIDES.ldap.title}
  subtitle={GUIDES.ldap.subtitle}
  backHref="/ldap"
  backLabel="Back to LDAP"
  crumb="LDAP Directory"
  doc={GUIDES.ldap.doc}
/>
