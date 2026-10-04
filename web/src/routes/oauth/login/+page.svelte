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

	let info = $state<AuthRequestInfo | null>(null);
	let error = $state('');
	let status = $state('Checking your all-in-one session…');

	// Runs the hand-off for an app login: if the user already has an aio
	// session, finish right away (single sign-on); otherwise send them through
	// aio's login or signup page and come back here.
	onMount(async () => {
		const id = $page.url.searchParams.get('authRequestID');
		if (!id) {
			error = 'This login link is missing its request. Go back to the app and try again.';
			return;
		}
		try {
			info = await getAuthRequest(id);
		} catch (err) {
			error = err instanceof Error ? err.message : 'This login link is not valid.';
			return;
		}

		const here = `/oauth/login?authRequestID=${encodeURIComponent(id)}`;
		if (!(await hasSession())) {
			const target = info.signup ? '/signup' : '/login';
			await goto(`${target}?next=${encodeURIComponent(here)}`, { replaceState: true });
			return;
		}

		status = `Continuing to ${info.client_name}…`;
		const result = await completeAuthRequest(id);
		if (result.kind === 'redirect') {
			window.location.replace(result.url);
		} else if (result.kind === 'login-required') {
			await goto(`/login?next=${encodeURIComponent(here)}`, { replaceState: true });
		} else {
			error = result.message;
		}
	});
</script>

<svelte:head>
	<title>{info ? `Continue to ${info.client_name}` : 'Log in'} · All-in-one</title>
</svelte:head>

<div class="flex min-h-[calc(100vh-3.5rem)] items-center justify-center p-4">
	<Card.Root class="w-full max-w-md">
		<Card.Header>
			<Card.Title class="text-2xl">
				{info ? `Continue to ${info.client_name}` : 'Log in with All-in-one'}
			</Card.Title>
			<Card.Description>
				{#if info}
					{info.client_name} uses your all-in-one account to log you in.
				{/if}
			</Card.Description>
		</Card.Header>
		<Card.Content>
			{#if error}
				<div class="rounded-md bg-destructive/15 p-3 text-sm text-destructive">{error}</div>
				<Button variant="outline" class="mt-4 w-full" onclick={() => history.back()}>Go back</Button>
			{:else}
				<div class="flex items-center gap-3 text-sm text-muted-foreground">
					<Loader2 class="size-4 animate-spin" />
					{status}
				</div>
			{/if}
		</Card.Content>
	</Card.Root>
</div>
