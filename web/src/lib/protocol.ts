// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

// Shared wireless helpers used across the networks pages.

export function protocolBadge(p: string): string {
	const map: Record<string, string> = {
		open: 'badge-open', wpa: 'badge-wpa', wpa2: 'badge-wpa2', wps: 'badge-wps',
		wpa3: 'badge-wpa3', wpa3_transition: 'badge-wpa3',
		wpa2_enterprise: 'badge-enterprise', wpa3_enterprise: 'badge-enterprise'
	};
	return map[p] ?? 'badge-neutral';
}
