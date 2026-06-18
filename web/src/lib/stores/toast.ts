// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

import { writable } from 'svelte/store';

export type ToastType = 'success' | 'err' | 'warning' | 'info';

export interface Toast {
	id: string;
	message: string;
	type: ToastType;
	duration?: number;
}

function createToastStore() {
	const { subscribe, update } = writable<Toast[]>([]);

	function add(message: string, type: ToastType = 'info', duration: number = 5000) {
		const id = Math.random().toString(36).substring(2, 9);
		const toast: Toast = { id, message, type, duration };

		update((toasts) => [...toasts, toast]);

		if (duration > 0) {
			setTimeout(() => remove(id), duration);
		}
	}

	function remove(id: string) {
		update((toasts) => toasts.filter((t) => t.id !== id));
	}

	return {
		subscribe,
		add,
		remove,
		success: (msg: string, dur?: number) => add(msg, 'success', dur),
		err: (msg: string, dur: number = 8000) => add(msg, 'err', dur),
		warning: (msg: string, dur?: number) => add(msg, 'warning', dur),
		info: (msg: string, dur?: number) => add(msg, 'info', dur)
	};
}

export const toast = createToastStore();
