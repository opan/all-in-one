import { goto } from '$app/navigation';
import { browser } from '$app/environment';
import { redirect } from '@sveltejs/kit';

/**
 * API client wrapper that handles authentication and redirects on 401
 * For use in browser/component context
 */
export async function apiClient(
	url: string,
	options: RequestInit = {}
): Promise<Response> {
	// Always include credentials for cookie-based auth
	const config: RequestInit = {
		...options,
		credentials: 'include',
		headers: {
			'Content-Type': 'application/json',
			...options.headers,
		},
	};

	try {
		const response = await fetch(url, config);

		// Handle 401 Unauthorized - redirect to login
		if (response.status === 401 && browser) {
			// Clear any local state if needed
			await goto('/login', { replaceState: true });
			throw new Error('Unauthorized - redirecting to login');
		}

		return response;
	} catch (error) {
		// Re-throw for caller to handle
		throw error;
	}
}

/**
 * API client wrapper for server-side load functions
 * Uses SvelteKit's fetch and handles redirects properly
 */
export async function apiLoad(
	fetchFn: typeof fetch,
	url: string,
	options: RequestInit = {}
): Promise<Response> {
	const config: RequestInit = {
		...options,
		credentials: 'include',
		headers: {
			'Content-Type': 'application/json',
			...options.headers,
		},
	};

	const response = await fetchFn(url, config);

	// Handle 401 Unauthorized - use SvelteKit redirect
	if (response.status === 401) {
		throw redirect(302, '/login');
	}

	return response;
}

/**
 * Convenience method for GET requests
 */
export async function apiGet(url: string): Promise<Response> {
	return apiClient(url, { method: 'GET' });
}

/**
 * Convenience method for POST requests
 */
export async function apiPost(url: string, data?: unknown): Promise<Response> {
	return apiClient(url, {
		method: 'POST',
		body: data ? JSON.stringify(data) : undefined,
	});
}

/**
 * Convenience method for PUT requests
 */
export async function apiPut(url: string, data?: unknown): Promise<Response> {
	return apiClient(url, {
		method: 'PUT',
		body: data ? JSON.stringify(data) : undefined,
	});
}

/**
 * Convenience method for DELETE requests
 */
export async function apiDelete(url: string): Promise<Response> {
	return apiClient(url, { method: 'DELETE' });
}
