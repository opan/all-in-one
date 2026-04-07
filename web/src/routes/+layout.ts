// Disable SSR globally — the Go backend provides the API; the frontend is
// a pure client-side SPA served as static files via adapter-static.
export const ssr = false;

export function load({ url, data }) {
	return {
		url: url.pathname,
		pageData: data,
	};
}
