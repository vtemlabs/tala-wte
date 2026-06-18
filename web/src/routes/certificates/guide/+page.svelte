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
  title={GUIDES.certificates.title}
  subtitle={GUIDES.certificates.subtitle}
  backHref="/certificates"
  backLabel="Back to Certificates"
  crumb="Certificates"
  doc={GUIDES.certificates.doc}
/>
