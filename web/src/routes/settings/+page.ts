// SvelteKit load function for settings page

export const load = async ({ fetch }) => {
	let user = null;

	try {
		const response = await fetch("/api/v1/users/me");
		if (response.ok) {
			const result = await response.json();
			user = result.data;
		}
	} catch (error) {
		console.error("Error fetching user data:", error);
	}

	return {
		breadcrumb: { label: "Settings", href: "/settings" },
		user,
	};
};
