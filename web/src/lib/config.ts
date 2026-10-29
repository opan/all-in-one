// Client-facing runtime config served by the Go backend at GET /api/v1/config.
// Read before authentication (e.g. to advertise the demo account), so it must
// degrade gracefully when the backend is unreachable.

export interface DemoMode {
	enabled: boolean;
	username?: string;
	password?: string;
}

export interface PublicConfig {
	demo_mode: DemoMode;
	// Password recovery (RFC-003): whether aio can email temporary passwords,
	// and who to write to when an account has no email.
	email_enabled: boolean;
	support_email: string;
	// aio's address as users see it, named on app login pages.
	account_host: string;
}

const DEMO_DISABLED: DemoMode = { enabled: false };

const CONFIG_UNAVAILABLE: PublicConfig = {
	demo_mode: DEMO_DISABLED,
	email_enabled: false,
	support_email: '',
	account_host: ''
};

// fetchPublicConfig reads GET /api/v1/config, falling back to "nothing
// enabled" on any error.
export async function fetchPublicConfig(fetchFn: typeof fetch = fetch): Promise<PublicConfig> {
	try {
		const res = await fetchFn('/api/v1/config', { credentials: 'include' });
		if (!res.ok) return CONFIG_UNAVAILABLE;
		const data = (await res.json())?.data ?? {};
		const demo = data.demo_mode;
		return {
			demo_mode: demo && typeof demo.enabled === 'boolean' ? demo : DEMO_DISABLED,
			email_enabled: data.email_enabled === true,
			support_email: typeof data.support_email === 'string' ? data.support_email : '',
			account_host: typeof data.account_host === 'string' ? data.account_host : ''
		};
	} catch {
		return CONFIG_UNAVAILABLE;
	}
}
