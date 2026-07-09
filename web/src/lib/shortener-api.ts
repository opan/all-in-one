import { apiClient, apiLoad, apiPost, apiDelete, throwApiError } from '$lib/api';

// Mirrors internal/shortener/model/shortlink.go JSON tags
export interface ShortLink {
	id: string;
	code: string;
	target_url: string;
	created_at: string;
	expires_at?: string | null;
	is_active: boolean;
	click_count: number;
	last_accessed_at?: string | null;
}

export interface CreateShortLinkPayload {
	target_url: string;
	expires_at?: string;
}

export interface ShortLinksPage {
	links: ShortLink[];
	total: number;
	page: number;
}

const BASE = '/api/v1/shortener/links';

export async function listShortLinks(page = 1, pageSize = 50): Promise<ShortLinksPage> {
	const res = await apiClient(`${BASE}?page=${page}&page_size=${pageSize}`);
	if (!res.ok) throw new Error('Failed to fetch short links');
	const body = await res.json();
	return (body.data ?? { links: [], total: 0, page }) as ShortLinksPage;
}

export async function createShortLink(payload: CreateShortLinkPayload): Promise<ShortLink> {
	const res = await apiPost(BASE, payload);
	const body = await res.json();
	if (!res.ok) throw new Error(body.error ?? 'Failed to create short link');
	return body.data as ShortLink;
}

export async function updateShortLink(
	code: string,
	patch: { is_active?: boolean; expires_at?: string }
): Promise<ShortLink> {
	const res = await apiClient(`${BASE}/${code}`, {
		method: 'PATCH',
		body: JSON.stringify(patch)
	});
	const body = await res.json();
	if (!res.ok) throw new Error(body.error ?? 'Failed to update short link');
	return body.data as ShortLink;
}

export async function deleteShortLink(code: string): Promise<void> {
	const res = await apiDelete(`${BASE}/${code}`);
	if (!res.ok) {
		const body = await res.json().catch(() => ({}));
		throw new Error((body as { error?: string }).error ?? 'Failed to delete short link');
	}
}

// Used by the +page.ts server-side load function
export async function loadShortLinks(fetchFn: typeof fetch): Promise<ShortLinksPage> {
	const res = await apiLoad(fetchFn, `${BASE}?page=1&page_size=50`);
	if (!res.ok) await throwApiError(res, 'Failed to fetch short links');
	const body = await res.json();
	return (body.data ?? { links: [], total: 0, page: 1 }) as ShortLinksPage;
}

export function shortURL(code: string): string {
	if (typeof window === 'undefined') return `/r/${code}`;
	return `${window.location.origin}/r/${code}`;
}
