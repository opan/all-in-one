import { apiPatch, apiPost } from '$lib/api';

// Admin user-management calls (authnz domain, /api/v1/admin/*). Access/RBAC
// calls live separately in rbac-api.ts.
const BASE = '/api/v1/admin/users';

async function expectOk(res: Response): Promise<void> {
	if (!res.ok) {
		const body = await res.json().catch(() => null);
		throw new Error(body?.error ?? 'Request failed');
	}
}

export async function updateUserEmail(userId: string, email: string): Promise<void> {
	await expectOk(await apiPatch(`${BASE}/${userId}`, { email }));
}

export async function blockUser(userId: string): Promise<void> {
	await expectOk(await apiPost(`${BASE}/${userId}/block`));
}

export async function unblockUser(userId: string): Promise<void> {
	await expectOk(await apiPost(`${BASE}/${userId}/unblock`));
}
