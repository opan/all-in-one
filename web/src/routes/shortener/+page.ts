import { loadShortLinks } from '$lib/shortener-api';

export const load = async ({ fetch }) => {
	const page = await loadShortLinks(fetch);
	return {
		links: page.links,
		total: page.total,
		breadcrumb: { label: 'Shortener' }
	};
};
