// Password recovery and email confirmation (RFC-003). Plain fetch, like the
// login page: apiClient redirects to /login on a 401, which would drop the
// ?next= these pages carry through an app login.

export interface ApiResult<T = unknown> {
	ok: boolean;
	status: number;
	error: string;
	message: string;
	data: T | null;
}

async function call<T>(method: string, url: string, body?: unknown): Promise<ApiResult<T>> {
	try {
		const res = await fetch(url, {
			method,
			credentials: 'include',
			headers: { 'Content-Type': 'application/json' },
			body: body === undefined ? undefined : JSON.stringify(body)
		});
		const json = await res.json().catch(() => null);
		return {
			ok: res.ok,
			status: res.status,
			error: json?.error ?? '',
			message: json?.message ?? '',
			data: (json?.data ?? null) as T | null
		};
	} catch {
		return { ok: false, status: 0, error: '', message: '', data: null };
	}
}

// AppContext names the app the user came from, so emails can mention it and
// use its language.
export interface AppContext {
	locale?: string;
	client_id?: string;
}

export const forgotPassword = (login: string, app: AppContext) =>
	call('POST', '/api/v1/auth/password/forgot', { login, ...app });

export const setPassword = (password: string, app: AppContext) =>
	call<{ email_prompt: boolean }>('POST', '/api/v1/users/password/set', { password, ...app });

export const updateMyEmail = (email: string, app: AppContext) =>
	call<{ pending_email: string }>('PUT', '/api/v1/users/me/email', { email, ...app });

export const verifyEmail = (token: string) =>
	call<{ email: string }>('POST', '/api/v1/auth/email/verify', { token });

export const snoozeEmailPrompt = () => call('POST', '/api/v1/users/me/email-prompt/snooze');

export interface Me {
	username: string;
	email: string;
	email_verified: boolean;
	pending_email?: string;
	password_change_required: boolean;
}

export const getMe = () => call<Me>('GET', '/api/v1/users/me');

export const changePassword = (currentPassword: string, password: string) =>
	call('POST', '/api/v1/users/reset_password', { current_password: currentPassword, password });

// afterLoginPath decides where a successful login goes next: choose a new
// password, add an email, or the page the login was for.
export function afterLoginPath(
	result: { password_change_required?: boolean; email_prompt?: boolean } | null,
	next: string | null,
	extraQuery = ''
): string {
	const target = next ?? '/home';
	const q = `next=${encodeURIComponent(target)}${extraQuery ? `&${extraQuery}` : ''}`;
	if (result?.password_change_required) return `/set-password?${q}`;
	if (result?.email_prompt) return `/add-email?${q}`;
	return target;
}

// signedInPath is where /login and /signup send a visitor who already has an
// aio session: the dashboard, or choosing a new password after a temporary
// one. null when not signed in, so the page shows its form. /users/me also
// answers for a session that must change its password first.
export async function signedInPath(): Promise<string | null> {
	let me = await getMe();
	if (me.status === 401 && (await call('POST', '/api/v1/sessions/refresh')).ok) me = await getMe();
	if (!me.ok) return null;
	return afterLoginPath({ password_change_required: me.data?.password_change_required }, null);
}
