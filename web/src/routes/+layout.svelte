<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { ModeWatcher } from 'mode-watcher';
	import favicon from '$lib/assets/favicon.svg';
	import AppLayout from "../components/app-layout.svelte";
	import { page } from '$app/stores';
	import { initPalette } from '$lib/stores/theme';

	let { children, data } = $props();

	onMount(() => {
		initPalette();
	});

	// Check if current route should use the bare layout (no app sidebar/shell).
	// This covers the auth pages and the public landing page at "/".
	const isBarePage = $derived(
		$page.url.pathname === '/' ||
			$page.url.pathname.startsWith('/login') ||
			$page.url.pathname.startsWith('/signup')
	);

	// Build breadcrumbs from page data breadcrumb metadata
	const breadcrumbs = $derived(() => {
		const pageData = $page.data;
		const crumbs: Array<{ label: string; href?: string }> = [
			{ label: 'All-in-one', href: '/home' }
		];

		// Collect breadcrumbs from the page data hierarchy
		// SvelteKit provides parent data through the load function chain
		if (pageData.breadcrumb) {
			// For nested routes, we need to build the breadcrumb trail
			const path = data.url;
			
			if (path.startsWith('/listing')) {
				// Add intermediate breadcrumbs based on the route depth
				if (path.startsWith('/listing/topics/') && path !== '/listing/topics') {
					// Topic detail page: add Topics breadcrumb before the topic name
					crumbs.push({ label: 'Topics', href: '/listing/topics' });
				}
				// Add the current page breadcrumb
				crumbs.push(pageData.breadcrumb);
			}
		}

		return crumbs;
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<ModeWatcher />

{#if isBarePage}
	<!-- Bare layout for auth pages and the public landing page (no sidebar) -->
	{@render children?.()}
{:else}
	<!-- Full layout with sidebar for app pages -->
	<div class="min-h-screen bg-background text-foreground">
		<AppLayout breadcrumbs={breadcrumbs()} currentPath={data.url}>
			{@render children?.()}
		</AppLayout>
	</div>
{/if}
