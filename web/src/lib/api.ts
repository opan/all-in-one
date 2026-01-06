import { goto } from "$app/navigation";
import { browser } from "$app/environment";
import { redirect } from "@sveltejs/kit";

/**
 * API client wrapper that handles authentication and redirects on 401
 * For use in browser/component context
 */
export async function apiClient(
	url: string,
	options: RequestInit = {},
): Promise<Response> {
	// Always include credentials for cookie-based auth
	const config: RequestInit = {
		...options,
		credentials: "include",
		headers: {
			"Content-Type": "application/json",
			...options.headers,
		},
	};

	try {
		const response = await fetch(url, config);

		// Handle 401 Unauthorized - try to refresh token first
		if (response.status === 401 && browser) {
			// Attempt to refresh the access token
			const refreshResponse = await fetch("/api/v1/sessions/refresh", {
				method: "POST",
				credentials: "include",
				headers: {
					"Content-Type": "application/json",
				},
			});

			// If refresh succeeds, retry the original request
			if (refreshResponse.ok) {
				const retryResponse = await fetch(url, config);
				return retryResponse;
			}

			// If refresh fails, redirect to login
			await goto("/login", { replaceState: true });
			throw new Error("Session expired - redirecting to login");
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
	options: RequestInit = {},
): Promise<Response> {
	const config: RequestInit = {
		...options,
		credentials: "include",
		headers: {
			"Content-Type": "application/json",
			...options.headers,
		},
	};

	const response = await fetchFn(url, config);

	// Handle 401 Unauthorized - try to refresh token first
	if (response.status === 401) {
		// Attempt to refresh the access token
		const refreshResponse = await fetchFn("/api/v1/sessions/refresh", {
			method: "POST",
			credentials: "include",
			headers: {
				"Content-Type": "application/json",
			},
		});

		// If refresh succeeds, retry the original request
		if (refreshResponse.ok) {
			const retryResponse = await fetchFn(url, config);
			return retryResponse;
		}

		// If refresh fails, use SvelteKit redirect
		throw redirect(302, "/login");
	}

	return response;
}

/**
 * Convenience method for GET requests
 */
export async function apiGet(url: string): Promise<Response> {
	return apiClient(url, { method: "GET" });
}

/**
 * Convenience method for POST requests
 */
export async function apiPost(url: string, data?: unknown): Promise<Response> {
	return apiClient(url, {
		method: "POST",
		body: data ? JSON.stringify(data) : undefined,
	});
}

/**
 * Convenience method for PUT requests
 */
export async function apiPut(url: string, data?: unknown): Promise<Response> {
	return apiClient(url, {
		method: "PUT",
		body: data ? JSON.stringify(data) : undefined,
	});
}

/**
 * Convenience method for DELETE requests
 */
export async function apiDelete(url: string): Promise<Response> {
	return apiClient(url, { method: "DELETE" });
}

/**
 * Check if user is currently logged in by verifying the session
 * Returns true if session is valid, false otherwise
 * If access token is expired, attempts to refresh it using the refresh token
 */
export async function isLoggedIn(): Promise<boolean> {
	try {
		// First, try to verify the current session
		const verifyResponse = await fetch("/api/v1/sessions/verify", {
			method: "GET",
			credentials: "include",
			headers: {
				"Content-Type": "application/json",
			},
		});

		// If verification succeeds, user is logged in
		if (verifyResponse.ok) {
			return true;
		}

		// If verification fails with 401, try to refresh the access token
		if (verifyResponse.status === 401) {
			const refreshResponse = await fetch("/api/v1/sessions/refresh", {
				method: "POST",
				credentials: "include",
				headers: {
					"Content-Type": "application/json",
				},
			});

			// If refresh succeeds, user is logged in
			if (refreshResponse.ok) {
				return true;
			}

			// If refresh fails, redirect to login page
			if (browser) {
				await goto("/login", { replaceState: true });
			}
			return false;
		}

		// For other errors, user is not logged in
		return false;
	} catch (error) {
		// On any error, user is not logged in
		return false;
	}
}
