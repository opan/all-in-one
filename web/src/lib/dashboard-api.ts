import { apiLoad } from '$lib/api';

// Mirrors internal/dashboard/model/summary.go JSON tags. A section is present
// only when the authenticated user can access that feature (server-side RBAC
// gate), so section presence — not a separate feature list — drives which
// launcher cards and stat tiles the home page renders.
export interface ListingStats {
	topics: number;
}

export interface ChatStats {
	conversations: number;
	pending_invites: number;
}

export interface ShortenerStats {
	links: number;
}

export interface DashboardSummary {
	listing?: ListingStats;
	chat?: ChatStats;
	shortener?: ShortenerStats;
}

// getDashboardSummary loads the home dashboard summary. It goes through apiLoad
// so an unauthenticated visitor is redirected to /login (after a token-refresh
// attempt). Any non-auth error yields an empty summary so the page still
// renders rather than erroring the whole route.
export async function getDashboardSummary(fetchFn: typeof fetch): Promise<DashboardSummary> {
	const res = await apiLoad(fetchFn, '/api/v1/dashboard/summary');
	if (!res.ok) {
		return {};
	}
	const body = await res.json();
	return (body.data ?? {}) as DashboardSummary;
}
