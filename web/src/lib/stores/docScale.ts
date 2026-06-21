// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. See the LICENSE file.

// Reader-controlled text scale for the in-app guides; persisted and shared across
// the floating Guide window and the full-page guide routes.
import { writable } from 'svelte/store';

const KEY = 'tala-doc-scale';
const MIN = 0.8;
const MAX = 1.8;
const clamp = (v: number) => Math.min(MAX, Math.max(MIN, Math.round(v * 100) / 100));

function read(): number {
	if (typeof localStorage === 'undefined') return 1;
	const v = parseFloat(localStorage.getItem(KEY) ?? '');
	return Number.isFinite(v) ? clamp(v) : 1;
}

export const docScale = writable(read());

if (typeof localStorage !== 'undefined') {
	docScale.subscribe((v) => localStorage.setItem(KEY, String(v)));
}

export function zoomDoc(delta: number) {
	docScale.update((v) => clamp(v + delta));
}
