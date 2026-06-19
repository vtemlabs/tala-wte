// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

export interface WirelessClient {
	mac: string;
	ip?: string;
	signal: number;
}

export interface WirelessInterface {
	interface: string;
	phy: string;
	driver: string;
	manufacturer?: string;
	device_model?: string;
	chipset?: string;
	bands?: string[];
	ap_bands?: string[]; // bands this card can host an AP on (subset of bands)
	limits?: string[]; // plain-language capability limits computed in discovery
}

export interface LDAPUser {
	dn: string;
	uid: string;
	cn: string;
	sn: string;
	mail: string;
	password?: string;
}

export interface LDAPGroup {
	dn: string;
	cn: string;
	members: string[];
}

export interface LDAPStatus {
	running: boolean;
	base_dn: string;
}

export interface TestAuthResult {
	success: boolean;
	message: string;
	dn?: string;
}
