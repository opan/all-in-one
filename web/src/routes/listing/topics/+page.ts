// SvelteKit load function to fetch topics data from backend API
import { apiLoad, throwApiError } from "#lib/api.js";

export const load = async ({ fetch }) => {
	const res = await apiLoad(fetch, "/api/v1/topics");
	if (!res.ok) {
		await throwApiError(res, "Failed to fetch topics");
	}
	const data = await res.json();
	return {
		topics: data.data || [],
		breadcrumb: { label: "Topics", href: "/listing/topics" },
	};
};
