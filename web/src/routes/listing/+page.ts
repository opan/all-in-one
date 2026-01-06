// SvelteKit load function to fetch listing data from backend API
import { apiLoad } from "$lib/api";

export const load = async ({ fetch }) => {
	const res = await apiLoad(fetch, "/api/v1/items");
	if (!res.ok) {
		throw new Error("Failed to fetch listings");
	}
	const data = await res.json();
	return {
		listings: data.data || [],
		breadcrumb: { label: "Listing" },
	};
};
