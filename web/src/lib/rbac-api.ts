import { apiGet, apiPost, apiPut, apiDelete } from '$lib/api';

// Mirrors internal/rbac/model/model.go JSON tags
export interface Feature {
	id: string;
	key: string;
	name: string;
	description?: string;
	admin_only: boolean;
	created_at?: string;
	updated_at?: string;
}

export interface Group {
	id: string;
	name: string;
	description?: string;
	is_builtin: boolean;
	created_at?: string;
	updated_at?: string;
	feature_keys?: string[];
}

export interface UserAccess {
	user_id: string;
	username: string;
	email: string;
	group_id: string | null;
	group_name: string | null;
	is_admin: boolean;
}

export interface FeatureOverride {
	feature_key: string;
	allow: boolean;
}

export interface GroupInput {
	name: string;
	description: string;
	feature_keys: string[];
}

const BASE = '/api/v1/access';

async function unwrap<T>(res: Response): Promise<T> {
	const body = await res.json();
	if (!res.ok) throw new Error(body.error ?? 'Request failed');
	return body.data as T;
}

export async function listFeatures(): Promise<Feature[]> {
	return unwrap<Feature[]>(await apiGet(`${BASE}/features`));
}

export async function listGroups(): Promise<Group[]> {
	return unwrap<Group[]>(await apiGet(`${BASE}/groups`));
}

export async function getGroup(id: string): Promise<Group> {
	return unwrap<Group>(await apiGet(`${BASE}/groups/${id}`));
}

export async function createGroup(input: GroupInput): Promise<Group> {
	return unwrap<Group>(await apiPost(`${BASE}/groups`, input));
}

export async function updateGroup(
	id: string,
	input: { name: string; description: string }
): Promise<Group> {
	return unwrap<Group>(await apiPut(`${BASE}/groups/${id}`, input));
}

export async function deleteGroup(id: string): Promise<void> {
	await unwrap<null>(await apiDelete(`${BASE}/groups/${id}`));
}

export async function setGroupFeatures(id: string, featureKeys: string[]): Promise<Group> {
	return unwrap<Group>(
		await apiPut(`${BASE}/groups/${id}/features`, { feature_keys: featureKeys })
	);
}

export async function listUsers(): Promise<UserAccess[]> {
	return unwrap<UserAccess[]>(await apiGet(`${BASE}/users`));
}

export async function assignUserGroup(userId: string, groupId: string | null): Promise<void> {
	await unwrap<null>(await apiPut(`${BASE}/users/${userId}/group`, { group_id: groupId }));
}

export async function getUserOverrides(userId: string): Promise<FeatureOverride[]> {
	return unwrap<FeatureOverride[]>(await apiGet(`${BASE}/users/${userId}/overrides`));
}

export async function setUserOverrides(
	userId: string,
	overrides: FeatureOverride[]
): Promise<void> {
	await unwrap<null>(
		await apiPut(`${BASE}/users/${userId}/overrides`, { overrides })
	);
}
