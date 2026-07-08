// Guard for all /admin/* routes: admins only. Runs client-side (ssr=false).
// The backend RequireAdmin middleware is the real enforcement; this just keeps
// non-admins from seeing the admin UI and bouncing off failed API calls.
import { redirect } from '@sveltejs/kit';

export const load = async ({ fetch }) => {
	let isAdmin = false;
	try {
		const res = await fetch('/api/v1/users/me');
		if (res.ok) {
			const body = await res.json();
			isAdmin = body.data?.is_admin === true;
		}
	} catch {
		// fall through to redirect
	}

	if (!isAdmin) {
		throw redirect(302, '/');
	}

	return {};
};
