// Disable SSR globally — the Go backend provides the API; the frontend is
// a pure client-side SPA served as static files via adapter-static.
import { fetchDemoMode } from '$lib/config';

export const ssr = false;

export async function load({ url, data, fetch }) {
	return {
		url: url.pathname,
		pageData: data,
		demoMode: await fetchDemoMode(fetch),
	};
}
