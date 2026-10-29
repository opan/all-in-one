<script lang="ts">
	import '../app.css';
	import { page } from '$app/state';
	import { Lock } from '@lucide/svelte/icons';
	import ThemeToggle from './theme-toggle.svelte';
	import { appForPage, appKeyForPage, type AppPresentation } from '#lib/oidc-api.js';
	import { authMessages } from '#lib/oauth-i18n.js';

	let { children } = $props();

	// When an app sent the user here, the page is presented as that app's
	// login (its name, icon, colour and language) with aio as the quiet
	// provider, so the hop from the app doesn't look like a different site.
	// The app comes from the login in progress, or from ?client_id= when the
	// app linked to a page directly (e.g. forgot password).
	const appKey = $derived(appKeyForPage(page.url));
	let app = $state<AppPresentation | null>(null);
	let failed = $state(false);

	$effect(() => {
		const key = appKey;
		app = null;
		failed = false;
		if (!key) return;

		appForPage(page.url).then((info) => key === appKey && (app = info)).catch(() => key === appKey && (failed = true));
	});

	// aio's address, from the server's settings: shown so users expect to
	// leave the app's address and know where their password belongs.
	const accountHost = $derived(page.data.publicConfig?.account_host as string | undefined || page.url.host);

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

{#if appKey && !failed}
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

		{#if app}
			<p
				class="mx-4 mt-4 flex max-w-md items-start gap-2 rounded-md border px-3 py-2 text-xs text-muted-foreground sm:mx-auto"
				data-testid="managed-notice"
			>
				<Lock class="mt-0.5 size-3 shrink-0" aria-hidden="true" />
				<span>
					{m.managedNotice(app.client_name, accountHost)}
					<strong class="text-foreground">{m.onlyHere}</strong>
				</span>
			</p>
		{/if}

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
