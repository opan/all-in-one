import { apiDelete, apiGet, apiPatch, apiPost } from '$lib/api';

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
	app: string;
	is_external: boolean;
}

export interface AppToken {
	id: string;
	app: string;
	name: string;
	token_prefix: string;
	scope_prefix: string;
	created_at: string;
	created_by?: string;
	last_used_at?: string;
	revoked_at?: string;
}

// CreateTokenResponse carries the plaintext token, shown exactly once.
export interface CreateTokenResponse extends AppToken {
	token: string;
	notice: string;
}

export interface CreateExternalTargetInput {
	key: string;
	app?: string;
	name?: string;
	description?: string;
	scope: Scope;
	kind: Kind;
	limit_count: number;
	window_value: number;
	window_unit: WindowUnit;
	enabled?: boolean;
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

export async function createExternalTarget(
	input: CreateExternalTargetInput
): Promise<RateLimitTarget> {
	return unwrap<RateLimitTarget>(await apiPost(`${BASE}/targets/external`, input));
}

export async function deleteExternalTarget(key: string): Promise<void> {
	await unwrap<null>(await apiDelete(`${BASE}/targets/${key}`));
}

export async function listTokens(): Promise<AppToken[]> {
	return unwrap<AppToken[]>(await apiGet(`${BASE}/tokens`));
}

export async function createToken(input: {
	app: string;
	name: string;
	scope_prefix?: string;
}): Promise<CreateTokenResponse> {
	return unwrap<CreateTokenResponse>(await apiPost(`${BASE}/tokens`, input));
}

export async function revokeToken(id: string): Promise<void> {
	await unwrap<null>(await apiDelete(`${BASE}/tokens/${id}`));
}
