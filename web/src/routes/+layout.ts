export function load({ url, data }) {
	return {
		url: url.pathname,
		pageData: data
	};
}
