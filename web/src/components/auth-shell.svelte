<script lang="ts">
	import '../app.css';
	import { page } from '$app/stores';
	import { Lock } from '@lucide/svelte/icons';
	import ThemeToggle from './theme-toggle.svelte';
	import { authRequestIdForPage, getAuthRequest, type AuthRequestInfo } from '$lib/oidc-api';
	import { authMessages } from '$lib/oauth-i18n';

	let { children } = $props();

	// When an app sent the user here, the page is presented as that app's
	// login (its name, icon, colour and language) with aio as the quiet
	// provider, so the hop from the app doesn't look like a different site.
	const requestId = $derived(authRequestIdForPage($page.url));
	let app = $state<AuthRequestInfo | null>(null);
	let failed = $state(false);

	$effect(() => {
		const id = requestId;
		app = null;
		failed = false;
		if (!id) return;
		getAuthRequest(id)
			.then((info) => id === requestId && (app = info))
			.catch(() => id === requestId && (failed = true));
	});

	$effect(() => {
		if (app?.locale) document.documentElement.lang = app.locale;
	});

	const m = $derived(authMessages(app?.locale));
	const brand = $derived(app?.brand_color || null);
	const onBrand = $derived(brand ? readableTextOn(brand) : null);

	// White or near-black text, whichever reads better on the brand colour.
	function readableTextOn(hex: string): string {
		const [r, g, b] = [1, 3, 5].map((i) => {
			const c = parseInt(hex.slice(i, i + 2), 16) / 255;
			return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
		});
		const luminance = 0.2126 * r + 0.7152 * g + 0.0722 * b;
		return luminance > 0.179 ? '#0b1020' : '#ffffff';
	}
</script>

{#if requestId && !failed}
	<div
		class="flex min-h-screen flex-col bg-background text-foreground"
		style:--primary={brand}
		style:--ring={brand}
		style:--primary-foreground={onBrand}
	>
		<header
			class="flex h-14 shrink-0 items-center gap-2 border-b px-4"
			style:background={brand}
			style:color={onBrand}
			data-testid="app-header"
		>
			{#if app}
				<span class="text-lg font-semibold">
					{#if app.icon}<span aria-hidden="true" class="mr-1">{app.icon}</span>{/if}{app.client_name}
				</span>
			{/if}
		</header>

		<main>
			{@render children?.()}
		</main>

		{#if app}
			<footer class="px-4 pt-6 pb-10 text-center text-xs text-muted-foreground">
				<p class="flex items-center justify-center gap-1">
					<Lock class="size-3" aria-hidden="true" />
					{m.securedBy}
				</p>
				<p class="mt-1">{m.securedByDetail(app.client_name)}</p>
			</footer>
		{/if}
	</div>
{:else}
	<div class="min-h-screen bg-background text-foreground">
		<header class="flex h-14 shrink-0 items-center gap-2 border-b px-4">
			<div class="flex-1">
				<a href="/" class="text-lg font-semibold">All-in-one</a>
			</div>

			<div class="ml-auto flex items-center gap-2">
				<ThemeToggle />
			</div>
		</header>

		<main class="flex-1">
			{@render children?.()}
		</main>
	</div>
{/if}
