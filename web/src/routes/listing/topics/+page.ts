// SvelteKit load function to fetch topics data from backend API
import { apiLoad } from '$lib/api';

export const load = async ({ fetch }) => {
  const res = await apiLoad(fetch, '/api/v1/topics');
  if (!res.ok) {
    throw new Error('Failed to fetch topics');
  }
  const data = await res.json();
  return {
    topics: data.data || [],
    breadcrumb: { label: 'Topics', href: '/listing/topics' }
  };
};
