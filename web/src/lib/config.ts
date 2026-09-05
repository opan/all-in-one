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
}

const DEMO_DISABLED: DemoMode = { enabled: false };

// fetchDemoMode returns the demo-account flag, defaulting to disabled on any
// error so a failed/blocked request never surfaces demo credentials.
export async function fetchDemoMode(
	fetchFn: typeof fetch = fetch,
): Promise<DemoMode> {
	try {
		const res = await fetchFn('/api/v1/config', { credentials: 'include' });
		if (!res.ok) return DEMO_DISABLED;
		const body = await res.json();
		const demo = body?.data?.demo_mode;
		return demo && typeof demo.enabled === 'boolean' ? demo : DEMO_DISABLED;
	} catch {
		return DEMO_DISABLED;
	}
}
