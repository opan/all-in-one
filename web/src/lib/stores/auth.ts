import { writable } from 'svelte/store';
import { browser } from '$app/environment';
import { goto } from '$app/navigation';
import { apiGet } from '$lib/api';

// Mirrors internal/authnz/handler.CurrentUserResponse JSON tags
export interface AuthUser {
	id: string;
	username: string;
	email: string;
	name?: string;
	is_admin: boolean;
	group: { id: string; name: string } | null;
	features: string[];
	email_verified: boolean;
	pending_email?: string;
	// The session was opened with a temporary password (RFC-003); everything
	// else is refused until the user chooses a new one.
	password_change_required: boolean;
}

export const auth = writable<AuthUser | null>(null);

// loadAuth fetches /users/me once and populates the shared store, replacing
// the 3+ ad-hoc /users/me fetches previously scattered across the sidebar,
// settings load, and chat page.
export async function loadAuth(): Promise<AuthUser | null> {
	try {
		const response = await apiGet('/api/v1/users/me');
		if (!response.ok) {
			auth.set(null);
			return null;
		}
		const result = await response.json();
		const user = (result.data ?? null) as AuthUser | null;
		auth.set(user);
		if (user?.password_change_required && browser) {
			const here = window.location.pathname + window.location.search;
			await goto(`/set-password?next=${encodeURIComponent(here)}`, { replaceState: true });
		}
		return user;
	} catch (error) {
		console.error('Failed to load current user:', error);
		auth.set(null);
		return null;
	}
}

// hasFeature reports whether user can access featureKey, per the sidebar's
// cosmetic filtering — the backend (RequireFeature/RequireAdmin middleware)
// is the actual enforcement point regardless of what this returns.
export function hasFeature(user: AuthUser | null, featureKey: string): boolean {
	if (!user) return false;
	if (user.is_admin) return true;
	return user.features.includes(featureKey);
}
