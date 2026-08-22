import { redirect } from "@sveltejs/kit";

// The listing feature lives at /listing/topics (topics) and
// /listing/topics/[id] (items). This bare /listing route used to be a stub
// wired to a nonexistent /api/v1/items endpoint; it's now just a redirect to
// the real entry point so any stray link/bookmark lands in the right place.
export const load = () => {
	throw redirect(307, "/listing/topics");
};
