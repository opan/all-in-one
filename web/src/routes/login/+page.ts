import { redirectIfSignedIn } from '#lib/account-api.js';

export const load = async ({ url }) => {
	await redirectIfSignedIn(url);
	return {};
};
