<script lang="ts">
	import '../app.css';
	import favicon from '$lib/assets/favicon.svg';
	import AppLayout from "../components/app-layout.svelte";

	let { children, data } = $props();

	const breadcrumbs = [
		{ label: 'Building Your Application', href: '/' },
		{ label: 'Data Fetching' }
	];
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<div class="min-h-screen bg-background text-foreground">
	<AppLayout {breadcrumbs} currentPath={data.url}>
		{@render children?.()}
	</AppLayout>
</div>
