// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

// Shared wireless helpers used across the networks pages.

export function protocolBadge(p: string): string {
	const map: Record<string, string> = {
		open: 'badge-open',
		wpa: 'badge-wpa',
		wpa2: 'badge-wpa2',
		wps: 'badge-wps',
		wpa3: 'badge-wpa3',
		wpa3_transition: 'badge-wpa3',
		wpa2_enterprise: 'badge-enterprise',
		wpa3_enterprise: 'badge-enterprise',
		owe: 'badge-open',
		wpa2_ft: 'badge-wpa2',
		owe_transition: 'badge-open'
	};
	return map[p] ?? 'badge-neutral';
}
