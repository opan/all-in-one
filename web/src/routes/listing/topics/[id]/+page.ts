// SvelteKit load function to fetch topic details and items from backend API
import { apiLoad, throwApiError } from "$lib/api";

export const load = async ({ params, fetch, parent }) => {
	const topicId = params.id;

	// Fetch topic details
	const topicRes = await apiLoad(fetch, `/api/v1/topics/${topicId}`);
	if (!topicRes.ok) {
		await throwApiError(topicRes, "Failed to fetch topic");
	}
	const topicData = await topicRes.json();

	// Fetch items for this topic
	const itemsRes = await apiLoad(fetch, `/api/v1/topics/${topicId}/items`);
	if (!itemsRes.ok) {
		await throwApiError(itemsRes, "Failed to fetch items");
	}
	const itemsData = await itemsRes.json();

	const topic = topicData.data || {};

	return {
		topic,
		items: itemsData.data || [],
		breadcrumb: { label: topic.name },
	};
};
