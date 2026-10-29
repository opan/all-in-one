import { appForPage, type AppPresentation } from '$lib/oidc-api';
import { authMessages, type AuthMessages } from '$lib/oauth-i18n';

// pageApp resolves the app a page belongs to (see appForPage) and the copy
// in its language. ready turns true once the lookup is done, so a page can
// wait instead of flashing English before the app's language.
export function pageApp(url: URL) {
	const state = $state<{ app: AppPresentation | null; ready: boolean }>({ app: null, ready: false });
	appForPage(url)
		.then((app) => (state.app = app))
		.catch(() => {})
		.finally(() => (state.ready = true));
	return {
		get app() {
			return state.app;
		},
		get ready() {
			return state.ready;
		},
		get m(): AuthMessages {
			return authMessages(state.app?.locale);
		},
		// For the API: the app's language and id, so emails can name it.
		get context() {
			return { locale: state.app?.locale || undefined, client_id: state.app?.client_id || undefined };
		}
	};
}
