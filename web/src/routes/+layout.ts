// Disable SSR globally — the Go backend provides the API; the frontend is
// a pure client-side SPA served as static files via adapter-static.
import { fetchPublicConfig } from '#lib/config.js';

export const ssr = false;

export async function load({ url, data, fetch }) {
	const publicConfig = await fetchPublicConfig(fetch);
	return {
		url: url.pathname,
		pageData: data,
		demoMode: publicConfig.demo_mode,
		publicConfig,
	};
}
