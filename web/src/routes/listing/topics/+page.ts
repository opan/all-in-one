// SvelteKit load function to fetch topics data from backend API
import { apiLoad } from '$lib/api';
import { error } from '@sveltejs/kit';

export const load = async ({ fetch }) => {
  // apiLoad will throw redirect(302, '/login') if auth fails
  // SvelteKit will handle the redirect automatically
  const res = await apiLoad(fetch, '/api/v1/topics');
  
  if (!res.ok) {
    // Check response body for error details
    const errorData = await res.json().catch(() => ({ error: 'Unknown error' }));
    throw error(res.status, errorData.error || 'Failed to fetch topics');
  }
  
  const data = await res.json();
  return {
    topics: data.data || [],
    breadcrumb: { label: 'Topics', href: '/listing/topics' }
  };
};
