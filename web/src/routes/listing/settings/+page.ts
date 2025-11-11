// SvelteKit load function for settings page

export const load = async () => {
  return {
    breadcrumb: { label: 'Settings', href: '/listing/settings' }
  };
};
