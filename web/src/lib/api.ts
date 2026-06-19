// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

import PocketBase, { type RecordSubscription } from 'pocketbase';

export const pb = new PocketBase(
	typeof window !== 'undefined' ? window.location.origin : 'http://localhost:8090'
);

// On 401/403 the token is invalid; clear it and force re-login.
pb.afterSend = (response, data) => {
	if (response.status === 401 || response.status === 403) {
		pb.authStore.clear();
		if (typeof window !== 'undefined') window.location.href = '/login';
	}
	return data;
};

async function handleResponse<T = any>(res: Response): Promise<T> {
	const data = await res.json();
	if (!res.ok) {
		throw new Error(data?.error || `HTTP ${res.status}`);
	}
	return data as T;
}

// Auth headers for raw fetch calls; the PocketBase SDK adds these itself.
function authHeaders(extra: Record<string, string> = {}): Record<string, string> {
	const headers: Record<string, string> = { ...extra };
	if (pb.authStore.token) {
		headers['Authorization'] = pb.authStore.token;
	}
	return headers;
}

// Networks
export const networks = {
	list: () => pb.collection('networks').getFullList(),

	get: (id: string) => pb.collection('networks').getOne(id),

	create: (data: Record<string, unknown>) =>
		pb.collection('networks').create({ ...data, status: 'stopped' }),

	update: (id: string, data: Record<string, unknown>) => pb.collection('networks').update(id, data),

	delete: (id: string) => pb.collection('networks').delete(id),

	start: async (
		id: string,
		opts: { autoProvision?: boolean; interface?: string; band?: string } = {}
	) => {
		const res = await fetch(`/api/wte/networks/${id}/start`, {
			method: 'POST',
			headers: authHeaders({ 'Content-Type': 'application/json' }),
			body: JSON.stringify({
				auto_provision: !!opts.autoProvision,
				interface: opts.interface ?? '',
				band: opts.band ?? ''
			})
		});
		const data = await res.json();
		// 412 means enterprise preflight failed; surface the structured payload.
		if (res.status === 412) {
			const err = new Error(data.error || 'Enterprise preflight failed') as Error & {
				preflight?: PreflightResult;
			};
			err.preflight = data.preflight;
			throw err;
		}
		// 409 + needs_adapter_choice: the saved radio is gone; surface the swap proposal.
		if (res.status === 409 && data.needs_adapter_choice) {
			const err = new Error(data.error || 'Configured adapter not available') as Error & {
				adapterSwap?: AdapterSwap;
			};
			err.adapterSwap = data;
			throw err;
		}
		if (!res.ok || data.error) throw new Error(data.error || 'Failed to start network');
		return data;
	},

	stop: async (id: string) => {
		const res = await fetch(`/api/wte/networks/${id}/stop`, {
			method: 'POST',
			headers: authHeaders()
		});
		const data = await res.json();
		if (!res.ok || data.error) throw new Error(data.error || 'Failed to stop network');
		return data;
	},

	clients: async (id: string) => {
		const res = await fetch(`/api/wte/networks/${id}/clients`, { headers: authHeaders() });
		return handleResponse(res);
	}
};

// Portals
export interface PortalTemplate {
	slug: string;
	name: string;
	category: string;
	description: string;
	html: string;
}

export const portals = {
	list: () => pb.collection('portals').getFullList({ sort: 'name' }),

	get: (id: string) => pb.collection('portals').getOne(id),

	create: (data: Record<string, unknown>) =>
		pb.collection('portals').create({ type: 'custom', category: 'custom', ...data }),

	update: (id: string, data: Record<string, unknown>) => pb.collection('portals').update(id, data),

	delete: (id: string) => pb.collection('portals').delete(id),

	templates: () =>
		fetch('/api/wte/portals/templates', { headers: authHeaders() })
			.then(handleResponse<{ templates: PortalTemplate[] }>)
			.then((r) => r.templates),

	// Re-seed built-in templates from the embedded source: recreate deleted ones
	// and reset changed ones to original. Custom portals are untouched.
	restore: () =>
		fetch('/api/wte/portals/restore', { method: 'POST', headers: authHeaders() }).then(
			handleResponse<{ restored: number; reset: number }>
		),

	// Upload a self-contained .html or a .zip bundle as a new custom portal.
	upload: async (file: File, name: string) => {
		const fd = new FormData();
		fd.append('file', file);
		fd.append('name', name);
		const res = await fetch('/api/wte/portals/upload', {
			method: 'POST',
			headers: authHeaders(),
			body: fd
		});
		return handleResponse<{ id: string; name: string }>(res);
	},

	// Clone a live page by URL; fetched SSRF-guarded server-side, with assets
	// inlined for offline use and the login wired to the accept endpoint.
	scrape: async (url: string, name: string) => {
		const res = await fetch('/api/wte/portals/scrape', {
			method: 'POST',
			headers: { ...authHeaders(), 'Content-Type': 'application/json' },
			body: JSON.stringify({ url, name })
		});
		return handleResponse<{ id: string; name: string }>(res);
	},

	previewURL: (id: string) => `/api/wte/portals/${id}/preview`
};

// Portal submissions (harvested credentials/PII)
export const submissions = {
	list: () => pb.collection('portal_submissions').getFullList({ sort: '-created' }),

	delete: (id: string) => pb.collection('portal_submissions').delete(id),

	subscribe: (cb: (data: RecordSubscription) => void) =>
		pb.collection('portal_submissions').subscribe('*', cb)
};

// Captures
export const captures = {
	list: () => pb.collection('captures').getFullList({ sort: '-started_at', expand: 'network_id' }),

	start: async (networkId: string, layer: 'wireless' | 'network', iface: string, filter = '') => {
		const res = await fetch('/api/wte/captures/start', {
			method: 'POST',
			headers: authHeaders({ 'Content-Type': 'application/json' }),
			body: JSON.stringify({ network_id: networkId, layer, interface: iface, filter })
		});
		return handleResponse(res);
	},

	stop: async (id: string) => {
		const res = await fetch(`/api/wte/captures/${id}/stop`, {
			method: 'POST',
			headers: authHeaders()
		});
		return handleResponse(res);
	},

	delete: (id: string) => pb.collection('captures').delete(id),

	downloadURL: (id: string) => `/api/wte/captures/${id}/download`,

	analyze: async (id: string) => {
		const res = await fetch(`/api/wte/captures/${id}/analyze`, { headers: authHeaders() });
		return handleResponse(res);
	},

	packets: async (id: string, filter = '', limit = 1000) => {
		const q = new URLSearchParams({ filter, limit: String(limit) });
		const res = await fetch(`/api/wte/captures/${id}/packets?${q}`, { headers: authHeaders() });
		return handleResponse(res);
	},

	packetDetail: async (id: string, n: number) => {
		const res = await fetch(`/api/wte/captures/${id}/packet/${n}`, { headers: authHeaders() });
		return handleResponse(res);
	},

	get: (id: string) => pb.collection('captures').getOne(id, { expand: 'network_id' })
};

// LDAP
export const ldap = {
	status: () => fetch('/api/wte/ldap/status', { headers: authHeaders() }).then(handleResponse),
	users: () => fetch('/api/wte/ldap/users', { headers: authHeaders() }).then(handleResponse),
	groups: () => fetch('/api/wte/ldap/groups', { headers: authHeaders() }).then(handleResponse),

	createUser: (data: object) =>
		fetch('/api/wte/ldap/users', {
			method: 'POST',
			headers: authHeaders({ 'Content-Type': 'application/json' }),
			body: JSON.stringify(data)
		}).then(handleResponse),

	deleteUser: (uid: string) =>
		fetch(`/api/wte/ldap/users/${uid}`, { method: 'DELETE', headers: authHeaders() }).then(
			handleResponse
		),

	testAuth: (uid: string, password: string) =>
		fetch('/api/wte/ldap/test-auth', {
			method: 'POST',
			headers: authHeaders({ 'Content-Type': 'application/json' }),
			body: JSON.stringify({ uid, password })
		}).then(handleResponse),

	createGroup: (cn: string, members: string[] = []) =>
		fetch('/api/wte/ldap/groups', {
			method: 'POST',
			headers: authHeaders({ 'Content-Type': 'application/json' }),
			body: JSON.stringify({ cn, members })
		}).then(handleResponse),

	deleteGroup: (cn: string) =>
		fetch(`/api/wte/ldap/groups/${encodeURIComponent(cn)}`, {
			method: 'DELETE',
			headers: authHeaders()
		}).then(handleResponse),

	provision: (data: {
		company_name: string;
		domain: string;
		user_count: number;
		random_passwords: boolean;
	}) =>
		fetch('/api/wte/ldap/provision', {
			method: 'POST',
			headers: authHeaders({ 'Content-Type': 'application/json' }),
			body: JSON.stringify(data)
		}).then(handleResponse),

	provisionRandom: () =>
		fetch('/api/wte/ldap/provision/random', { method: 'POST', headers: authHeaders() }).then(
			handleResponse
		)
};

// System
export interface VersionStatus {
	current: string;
	latest: string;
	update_available: boolean;
	notes: string;
	release_url: string;
	is_dev: boolean;
	error?: string;
}

export const system = {
	interfaces: () =>
		fetch('/api/wte/system/interfaces', { headers: authHeaders() }).then(handleResponse),
	status: () => fetch('/api/wte/system/status').then(handleResponse),

	// USB-reset recovery for a wedged adapter. Target by interface name (a radio
	// with a netdev) or usb_path (a device that enumerated but never initialized).
	heal: (target: { interface?: string; usb_path?: string }) =>
		fetch('/api/wte/system/interfaces/heal', {
			method: 'POST',
			headers: authHeaders({ 'Content-Type': 'application/json' }),
			body: JSON.stringify(target)
		}).then(handleResponse<{ ok: boolean; message: string }>),

	getSettings: () =>
		fetch('/api/wte/system/settings', { headers: authHeaders() }).then(handleResponse),

	saveSettings: (data: { uplink_iface?: string; country_code?: string; ap_subnet?: string }) =>
		fetch('/api/wte/system/settings', {
			method: 'POST',
			headers: authHeaders({ 'Content-Type': 'application/json' }),
			body: JSON.stringify(data)
		}).then(handleResponse),

	// Current version plus, when GitHub is reachable, whether a newer release exists.
	version: () =>
		fetch('/api/wte/system/version', { headers: authHeaders() }).then(
			handleResponse<VersionStatus>
		),

	// Download, verify, and install the latest release, then restart the service.
	update: () =>
		fetch('/api/wte/system/update', { method: 'POST', headers: authHeaders() }).then(
			handleResponse<{ status: string; version: string; restarting: boolean; message: string }>
		)
};

// Enterprise preflight
export interface PreflightCheck {
	id: string;
	label: string;
	ok: boolean;
	detail?: string;
	auto_fixable: boolean;
}
export interface PreflightResult {
	ok: boolean;
	checks: PreflightCheck[];
}
export interface AdapterSwap {
	needs_adapter_choice: true;
	error: string;
	missing: string;
	proposed: { interface: string; label: string };
	current_band: string;
	band_ok: boolean;
	suggested_band: string;
	band_reason: string;
}
export interface ProvisionStep {
	id: string;
	label: string;
	status: 'created' | 'skipped' | 'failed';
	detail?: string;
}
export interface ProvisionResult {
	ok: boolean;
	steps: ProvisionStep[];
	users?: { uid: string; cn: string; mail: string; password: string }[];
}

export const enterprise = {
	preflight: () =>
		fetch('/api/wte/enterprise/preflight', { headers: authHeaders() }).then(
			handleResponse<PreflightResult>
		),
	provision: () =>
		fetch('/api/wte/enterprise/provision', { method: 'POST', headers: authHeaders() }).then(
			handleResponse<ProvisionResult>
		)
};

// RADIUS config
export const radius = {
	saveConfig: (data: { eap_type: string; inner_auth: string; shared_secret: string }) =>
		fetch('/api/wte/radius/config', {
			method: 'POST',
			headers: authHeaders({ 'Content-Type': 'application/json' }),
			body: JSON.stringify(data)
		}).then(handleResponse)
};

// Certificates
export const certificates = {
	list: () => pb.collection('certificates').getFullList({ sort: 'name' }),

	createCA: () =>
		fetch('/api/wte/certs/ca', { method: 'POST', headers: authHeaders() }).then(handleResponse),

	createServer: (name: string) =>
		fetch(`/api/wte/certs/server?name=${encodeURIComponent(name)}`, {
			method: 'POST',
			headers: authHeaders()
		}).then(handleResponse),

	createClient: (uid: string) =>
		fetch(`/api/wte/certs/client?uid=${encodeURIComponent(uid)}`, {
			method: 'POST',
			headers: authHeaders()
		}).then(handleResponse)
};
