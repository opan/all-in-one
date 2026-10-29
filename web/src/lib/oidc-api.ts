// Login hand-off for apps that log users in through aio (OpenID Connect,
// RFC-001). Uses plain fetch on purpose: apiClient redirects to /login on a
// 401, which would drop the ?next= target this flow depends on.

import { safeNext } from '#lib/safe-next.js';

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
	// The session was opened with a temporary password (RFC-003).
	| { kind: 'password-change' }
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
	if (res.status === 403) {
		const body = await res.clone().json().catch(() => null);
		if (body?.error === 'password_change_required') return { kind: 'password-change' };
	}
	if (!res.ok) return { kind: 'error', message: await errorMessage(res, '') };
	return { kind: 'redirect', url: (await res.json()).data.redirect_url as string };
}

// authRequestIdFromNext extracts the auth request id when a login/signup page
// was opened from the OIDC hand-off, so it can show which app is asking.
export function authRequestIdFromNext(next: string | null): string | null {
	if (!next?.startsWith('/oauth/login?')) return null;
	return new URLSearchParams(next.slice('/oauth/login?'.length)).get('authRequestID');
}

// PageURL is the part of a page URL these helpers read. SvelteKit's page.url
// is readonly, so they take this rather than a mutable URL.
type QueryParams = Pick<URLSearchParams, 'get' | 'toString'>;
export interface PageURL {
	readonly pathname: string;
	readonly searchParams: QueryParams;
}

// authRequestIdForPage finds the app login a page belongs to: the hand-off
// page carries it directly, login and signup carry it inside ?next=.
export function authRequestIdForPage(url: PageURL): string | null {
	if (url.pathname.startsWith('/oauth/login')) return url.searchParams.get('authRequestID');
	return authRequestIdFromNext(safeNext(url.searchParams.get('next')));
}

// AppPresentation is what aio's pages need to present themselves as an app:
// from an auth request during a login, or from the registered client when
// the app links to a page directly (forgot password, account settings).
export interface AppPresentation {
	client_id: string;
	client_name: string;
	brand_color: string;
	icon: string;
	// "en" | "id" | ""
	locale: string;
	// Where to send the user back to in the app; checked by aio against the
	// app's registered addresses. Only for direct links.
	return_to?: string;
}

const apps = new Map<string, Promise<AppPresentation>>();

// getAppBranding loads a registered app for a page the app linked to:
// /forgot-password?client_id=cashflow&return_to=…&ui_locales=id
export function getAppBranding(clientId: string, returnTo: string, locale: string): Promise<AppPresentation> {
	const key = `${clientId}\n${returnTo}\n${locale}`;
	let p = apps.get(key);
	if (!p) {
		const q = new URLSearchParams({ return_to: returnTo });
		p = fetch(`/api/v1/oidc/apps/${encodeURIComponent(clientId)}?${q}`, { credentials: 'include' }).then(
			async (res) => {
				if (!res.ok) throw new Error('unknown app');
				const data = (await res.json()).data;
				return { ...data, locale } as AppPresentation;
			}
		);
		p.catch(() => apps.delete(key));
		apps.set(key, p);
	}
	return p;
}

// supportedLocale keeps the first of ui_locales that aio's pages are written in.
function supportedLocale(uiLocales: string | null): string {
	for (const tag of (uiLocales ?? '').split(/\s+/)) {
		const lang = tag.toLowerCase().split('-')[0];
		if (lang === 'en' || lang === 'id') return lang;
	}
	return '';
}

// directAppParams finds a direct link's ?client_id= (and return_to,
// ui_locales) on the page itself or inside its ?next= (e.g. the login page
// in front of /account?client_id=cashflow).
function directAppParams(url: PageURL): QueryParams | null {
	if (url.searchParams.get('client_id')) return url.searchParams;
	const next = safeNext(url.searchParams.get('next'));
	if (!next) return null;
	const inner = new URL(next, 'http://next.invalid').searchParams;
	return inner.get('client_id') ? inner : null;
}

// appKeyForPage identifies the app a page belongs to, or null for aio's own
// pages; it changes whenever the app does.
export function appKeyForPage(url: PageURL): string | null {
	const requestId = authRequestIdForPage(url);
	if (requestId) return `request:${requestId}`;
	const direct = directAppParams(url);
	return direct ? `app:${direct.toString()}` : null;
}

// appForPage finds the app a page belongs to, if any: the auth request of a
// login in progress (directly or inside ?next=), else a direct link's
// ?client_id=.
export async function appForPage(url: PageURL): Promise<AppPresentation | null> {
	const requestId = authRequestIdForPage(url);
	if (requestId) return getAuthRequest(requestId);
	const direct = directAppParams(url);
	if (!direct) return null;
	return getAppBranding(
		direct.get('client_id') ?? '',
		direct.get('return_to') ?? '',
		supportedLocale(direct.get('ui_locales'))
	);
}

// appQuery carries a direct link's app parameters on to the next page.
export function appQuery(url: PageURL): string {
	const q = new URLSearchParams();
	for (const key of ['client_id', 'return_to', 'ui_locales']) {
		const v = url.searchParams.get(key);
		if (v) q.set(key, v);
	}
	return q.toString();
}
