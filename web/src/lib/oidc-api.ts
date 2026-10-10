// Login hand-off for apps that log users in through aio (OpenID Connect,
// RFC-001). Uses plain fetch on purpose: apiClient redirects to /login on a
// 401, which would drop the ?next= target this flow depends on.

import { safeNext } from '$lib/safe-next';

export interface AuthRequestInfo {
	id: string;
	client_id: string;
	client_name: string;
	// How to present the app: "#rrggbb" and a short icon (emoji); "" = aio's look.
	brand_color: string;
	icon: string;
	// The app's ui_locales choice that aio supports ("en" | "id"), or "".
	locale: string;
	signup: boolean;
}

export type CompleteResult =
	| { kind: 'redirect'; url: string }
	| { kind: 'login-required' }
	| { kind: 'error'; message: string };

const BASE = '/api/v1/oidc/auth-requests';

// errorMessage turns an API error into a sentence for the page (the API's
// messages are lowercase, Go style).
async function errorMessage(res: Response, fallback: string): Promise<string> {
	const body = await res.json().catch(() => null);
	const msg: string = body?.error || fallback;
	return msg.charAt(0).toUpperCase() + msg.slice(1);
}

async function fetchAuthRequest(id: string): Promise<AuthRequestInfo> {
	const res = await fetch(`${BASE}/${encodeURIComponent(id)}`, { credentials: 'include' });
	if (!res.ok) throw new Error(await errorMessage(res, 'This login link is not valid.'));
	return (await res.json()).data as AuthRequestInfo;
}

// The page frame (branding) and the page itself both need the request; they
// share one fetch per id.
const requests = new Map<string, Promise<AuthRequestInfo>>();

export function getAuthRequest(id: string): Promise<AuthRequestInfo> {
	let p = requests.get(id);
	if (!p) {
		p = fetchAuthRequest(id);
		p.catch(() => requests.delete(id));
		requests.set(id, p);
	}
	return p;
}

// hasSession reports whether the browser has a usable aio session, refreshing
// an expired access token once. Never redirects.
export async function hasSession(): Promise<boolean> {
	const verify = await fetch('/api/v1/sessions/verify', { credentials: 'include' });
	if (verify.ok) return true;
	if (verify.status !== 401) return false;
	const refresh = await fetch('/api/v1/sessions/refresh', { method: 'POST', credentials: 'include' });
	return refresh.ok;
}

// locale asks aio for its error messages in the app's language.
export async function completeAuthRequest(id: string, locale?: string): Promise<CompleteResult> {
	const res = await fetch(`${BASE}/${encodeURIComponent(id)}/complete`, {
		method: 'POST',
		credentials: 'include',
		headers: locale ? { 'Accept-Language': locale } : {}
	});
	if (res.status === 401) return { kind: 'login-required' };
	if (!res.ok) return { kind: 'error', message: await errorMessage(res, '') };
	return { kind: 'redirect', url: (await res.json()).data.redirect_url as string };
}

// authRequestIdFromNext extracts the auth request id when a login/signup page
// was opened from the OIDC hand-off, so it can show which app is asking.
export function authRequestIdFromNext(next: string | null): string | null {
	if (!next?.startsWith('/oauth/login?')) return null;
	return new URLSearchParams(next.slice('/oauth/login?'.length)).get('authRequestID');
}

// authRequestIdForPage finds the app login a page belongs to: the hand-off
// page carries it directly, login and signup carry it inside ?next=.
export function authRequestIdForPage(url: URL): string | null {
	if (url.pathname.startsWith('/oauth/login')) return url.searchParams.get('authRequestID');
	return authRequestIdFromNext(safeNext(url.searchParams.get('next')));
}
