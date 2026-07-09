import { apiClient, apiPatch, apiDelete } from '$lib/api';
import type { ShortLink } from '$lib/shortener-api';

// Admin shortener moderation calls (/api/v1/admin/shortener/*). Unlike
// shortener-api.ts, these are owner-agnostic — list-all across every user.
const BASE = '/api/v1/admin/shortener/links';

export interface AdminShortLink extends ShortLink {
	owner_id?: string;
	owner_username?: string;
}

export interface AdminShortLinksPage {
	links: AdminShortLink[];
	total: number;
	page: number;
}

async function expectOk(res: Response): Promise<void> {
	if (!res.ok) {
		const body = await res.json().catch(() => null);
		throw new Error(body?.error ?? 'Request failed');
	}
}

export async function listAllShortLinks(page = 1, pageSize = 100): Promise<AdminShortLinksPage> {
	const res = await apiClient(`${BASE}?page=${page}&page_size=${pageSize}`);
	if (!res.ok) throw new Error('Failed to fetch short links');
	const body = await res.json();
	const data = body.data ?? { links: [], total: 0, page };
	return { ...data, links: data.links ?? [] } as AdminShortLinksPage;
}

export async function adminSetShortLinkActive(code: string, active: boolean): Promise<void> {
	await expectOk(await apiPatch(`${BASE}/${code}`, { is_active: active }));
}

export async function adminDeleteShortLink(code: string): Promise<void> {
	await expectOk(await apiDelete(`${BASE}/${code}`));
}
