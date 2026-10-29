<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import * as Card from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import { verifyEmail } from '$lib/account-api';
	import { authMessages } from '$lib/oauth-i18n';

	// Opened from the confirmation email (RFC-003 §6.2); works without being
	// logged in. The page follows the browser's language (en or id).
	const lang = typeof navigator !== 'undefined' && navigator.language.toLowerCase().startsWith('id') ? 'id' : 'en';
	const m = authMessages(lang);

	let status = $state<'checking' | 'confirmed' | 'failed'>('checking');
	let confirmedEmail = $state('');

	onMount(async () => {
		const token = $page.url.searchParams.get('token');
		// Drop the token from the address bar and history once read.
		history.replaceState(history.state, '', '/verify-email');
		if (!token) {
			status = 'failed';
			return;
		}
		const res = await verifyEmail(token);
		if (res.ok) {
			confirmedEmail = res.data?.email ?? '';
			status = 'confirmed';
		} else {
			status = 'failed';
		}
	});
</script>

<svelte:head>
	<title>{m.verifyTitle}</title>
</svelte:head>

<div class="flex items-start justify-center px-4 pt-10 sm:pt-16">
	<Card.Root class="w-full max-w-md">
		<Card.Header>
			<Card.Title class="text-2xl">{m.verifyTitle}</Card.Title>
			<Card.Description class="mt-2" data-testid="verify-status">
				{#if status === 'checking'}
					{m.verifyingEmail}
				{:else if status === 'confirmed'}
					{m.emailConfirmed(confirmedEmail)}
				{:else}
					{m.verifyFailed}
				{/if}
			</Card.Description>
		</Card.Header>
		{#if status !== 'checking'}
			<Card.Content>
				<Button href="/settings" class="w-full">{m.goToAccount}</Button>
			</Card.Content>
		{/if}
	</Card.Root>
</div>
