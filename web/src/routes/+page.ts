// Home dashboard load. Fetching the summary through apiLoad also enforces the
// auth gate: an unauthenticated visitor is redirected to /login.
import { getDashboardSummary } from '$lib/dashboard-api';

export const load = async ({ fetch }) => {
	const summary = await getDashboardSummary(fetch);
	return { summary };
};
