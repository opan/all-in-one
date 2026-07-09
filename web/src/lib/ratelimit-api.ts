import { apiGet, apiPatch, apiPost } from '$lib/api';

// Mirrors internal/ratelimit/model/model.go JSON tags
export type Scope = 'ip' | 'user' | 'global';
export type Kind = 'throttle' | 'daily_quota';
export type WindowUnit = 'second' | 'minute' | 'hour' | 'day';

export interface RateLimitTarget {
	key: string;
	name: string;
	description?: string;
	scope: Scope;
	kind: Kind;
	method: string;
	path: string;
	enabled: boolean;
	limit_count: number;
	window_value: number;
	window_unit: WindowUnit;
	updated_at?: string;
	updated_by?: string;
}

export interface TargetPatch {
	enabled?: boolean;
	limit_count?: number;
	window_value?: number;
	window_unit?: WindowUnit;
}

const BASE = '/api/v1/ratelimit';

async function unwrap<T>(res: Response): Promise<T> {
	const body = await res.json();
	if (!res.ok) throw new Error(body.error ?? 'Request failed');
	return body.data as T;
}

export async function listTargets(): Promise<RateLimitTarget[]> {
	return unwrap<RateLimitTarget[]>(await apiGet(`${BASE}/targets`));
}

export async function updateTarget(key: string, patch: TargetPatch): Promise<RateLimitTarget> {
	return unwrap<RateLimitTarget>(await apiPatch(`${BASE}/targets/${key}`, patch));
}

export async function resetCounters(key: string): Promise<void> {
	await unwrap<null>(await apiPost(`${BASE}/targets/${key}/reset`));
}

export async function resetDefaults(key: string): Promise<RateLimitTarget> {
	return unwrap<RateLimitTarget>(await apiPost(`${BASE}/targets/${key}/reset-defaults`));
}
