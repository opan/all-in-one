// SvelteKit load function to fetch listing data from backend API
import { apiLoad, throwApiError } from "$lib/api";

export const load = async ({ fetch }) => {
	const res = await apiLoad(fetch, "/api/v1/items");
	if (!res.ok) {
		await throwApiError(res, "Failed to fetch listings");
	}
	const data = await res.json();
	return {
		listings: data.data || [],
		breadcrumb: { label: "Listing" },
	};
};
