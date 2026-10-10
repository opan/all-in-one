<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import * as Card from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import { Loader2 } from '@lucide/svelte/icons';
	import {
		completeAuthRequest,
		getAuthRequest,
		hasSession,
		type AuthRequestInfo
	} from '$lib/oidc-api';
	import { authMessages } from '$lib/oauth-i18n';

	let info = $state<AuthRequestInfo | null>(null);
	let canSwitchAccount = $state(false);
	let here = '';
	let error = $state('');
	let continuing = $state(false);
	const m = $derived(authMessages(info?.locale));

	// Runs the hand-off for an app login: if the user already has an aio
	// session, finish right away (single sign-on); otherwise send them through
	// aio's login or signup page and come back here.
	onMount(async () => {
		const id = $page.url.searchParams.get('authRequestID');
		if (!id) {
			error = m.missingRequest;
			return;
		}
		try {
			info = await getAuthRequest(id);
		} catch (err) {
			error = err instanceof Error && err.message ? err.message : m.invalidLink;
			return;
		}

		here = `/oauth/login?authRequestID=${encodeURIComponent(id)}`;
		if (!(await hasSession())) {
			const target = info.signup ? '/signup' : '/login';
			await goto(`${target}?next=${encodeURIComponent(here)}`, { replaceState: true });
			return;
		}

		continuing = true;
		const result = await completeAuthRequest(id, info.locale);
		if (result.kind === 'redirect') {
			window.location.replace(result.url);
		} else if (result.kind === 'login-required') {
			await goto(`/login?next=${encodeURIComponent(here)}`, { replaceState: true });
		} else {
			error = result.message || m.couldNotFinish;
			// e.g. logged in to aio as the shared demo account: let the user log
			// out of it and continue with their own account.
			canSwitchAccount = true;
		}
	});

	async function switchAccount() {
		await fetch('/api/v1/sessions', { method: 'DELETE', credentials: 'include' });
		await goto(`/login?next=${encodeURIComponent(here)}`, { replaceState: true });
	}
</script>

<svelte:head>
	<title>{info ? m.handoffTitle(info.client_name) : `${m.handoffFallbackTitle}`}</title>
</svelte:head>

<div class="flex items-start justify-center px-4 pt-10 sm:pt-16">
	<Card.Root class="w-full max-w-md">
		<Card.Header>
			<Card.Title class="text-2xl">
				{info ? m.handoffTitle(info.client_name) : m.handoffFallbackTitle}
			</Card.Title>
			<Card.Description>
				{#if info}
					{m.handoffDescription(info.client_name)}
				{/if}
			</Card.Description>
		</Card.Header>
		<Card.Content>
			{#if error}
				<div class="rounded-md bg-destructive/15 p-3 text-sm text-destructive">{error}</div>
				{#if canSwitchAccount}
					<Button class="mt-4 w-full" onclick={switchAccount}>{m.useAnotherAccount}</Button>
				{/if}
				<Button variant="outline" class="mt-2 w-full" onclick={() => history.back()}>{m.goBack}</Button>
			{:else}
				<div class="flex items-center gap-3 text-sm text-muted-foreground">
					<Loader2 class="size-4 animate-spin" />
					{info && continuing ? m.continuingTo(info.client_name) : m.checkingSession}
				</div>
			{/if}
		</Card.Content>
	</Card.Root>
</div>
