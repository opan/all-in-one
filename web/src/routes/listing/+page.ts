// SvelteKit load function to fetch listing data from backend API
import { apiLoad, throwApiError } from "$lib/api";

export const load = async ({ fetch }) => {
	// NOTE: stopgap — the backend registers no `/api/v1/items`; the listing app is
	// topic-based (`/api/v1/topics`). Point the load at the real endpoint so the page
	// renders instead of 500-ing on the SPA-fallback HTML. The wider items-vs-topics
	// disconnect (page still renders hardcoded placeholders; CRUD hits `/items`) is
	// tracked in .context/LISTING_BACKEND_DISCONNECT.md.
	const res = await apiLoad(fetch, "/api/v1/topics");
	if (!res.ok) {
		await throwApiError(res, "Failed to fetch listings");
	}
	const data = await res.json();
	return {
		listings: data.data || [],
		breadcrumb: { label: "Listing" },
	};
};
