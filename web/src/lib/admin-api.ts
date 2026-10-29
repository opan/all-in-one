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

// emailVerified saves the address as verified: only when the user wrote to
// you from it (RFC-003 §6.5).
export async function updateUserEmail(userId: string, email: string, emailVerified = false): Promise<void> {
	await expectOk(await apiPatch(`${BASE}/${userId}`, { email, email_verified: emailVerified }));
}

// sendTempPassword emails the user a temporary password; the response never
// contains it. Returns the server's message ("sent to b•••@example.com").
export async function sendTempPassword(userId: string): Promise<string> {
	const res = await apiPost(`${BASE}/${userId}/password-reset`, {});
	const body = await res.json().catch(() => null);
	if (!res.ok) throw new Error(body?.error ?? 'Request failed');
	return body?.message ?? 'Temporary password sent';
}

export async function blockUser(userId: string): Promise<void> {
	await expectOk(await apiPost(`${BASE}/${userId}/block`));
}

export async function unblockUser(userId: string): Promise<void> {
	await expectOk(await apiPost(`${BASE}/${userId}/unblock`));
}
