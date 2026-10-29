<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Card from '$lib/components/ui/card';
	import { safeNext } from '$lib/safe-next';
	import { getMe, snoozeEmailPrompt, updateMyEmail } from '$lib/account-api';
	import { pageApp } from '$lib/use-app.svelte';

	// Shown after login to users without a confirmed email (RFC-003 §6.6).
	// Skippable: "Later" asks again in 7 days.
	const page$ = pageApp($page.url);
	const m = $derived(page$.m);
	const next = safeNext($page.url.searchParams.get('next')) ?? '/home';

	let checked = $state(false);
	let email = $state('');
	let sentTo = $state('');
	let loading = $state(false);
	let error = $state('');

	onMount(async () => {
		const me = await getMe();
		if (me.status === 401) {
			await goto(`/login?next=${encodeURIComponent(next)}`, { replaceState: true });
			return;
		}
		if (me.data?.email_verified) {
			await goto(next, { replaceState: true });
			return;
		}
		email = me.data?.pending_email || me.data?.email || '';
		checked = true;
	});

	const continueOn = () => goto(next, { replaceState: true });

	async function later() {
		loading = true;
		await snoozeEmailPrompt();
		await continueOn();
	}

	async function submit(event: Event) {
		event.preventDefault();
		error = '';
		loading = true;
		const res = await updateMyEmail(email.trim(), page$.context);
		loading = false;
		if (res.ok) {
			sentTo = res.data?.pending_email ?? email.trim();
			return;
		}
		switch (res.status) {
			case 400:
				error = res.error === 'this is already your email' ? '' : m.invalidEmail;
				if (!error) await continueOn();
				break;
			case 409:
				error = m.emailCantBeUsed;
				break;
			case 429:
				error = m.tooManyEmails;
				break;
			case 503:
				await continueOn();
				break;
			default:
				error = m.emailNotSent;
		}
	}
</script>

<svelte:head>
	<title>{m.addEmailTitle}</title>
</svelte:head>

{#if page$.ready && checked}
	<div class="flex items-start justify-center px-4 pt-10 sm:pt-16">
		<Card.Root class="w-full max-w-md">
			<Card.Header>
				<Card.Title class="text-2xl">{m.addEmailTitle}</Card.Title>
				<Card.Description class="mt-2">
					{sentTo ? m.confirmationSent(sentTo) : m.addEmailSubtitle}
				</Card.Description>
			</Card.Header>
			<Card.Content>
				{#if sentTo}
					<Button class="w-full" onclick={continueOn}>{m.continueButton}</Button>
				{:else}
					<form onsubmit={submit} class="space-y-4">
						{#if error}
							<div
								class="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-600 dark:border-red-800 dark:bg-red-950 dark:text-red-400"
							>
								{error}
							</div>
						{/if}
						<div class="space-y-2">
							<Label for="email">{m.email}</Label>
							<Input
								id="email"
								type="email"
								autocomplete="email"
								placeholder={m.emailPlaceholder}
								bind:value={email}
								disabled={loading}
								required
							/>
						</div>
						<div class="flex gap-2">
							<Button type="button" variant="outline" class="flex-1" onclick={later} disabled={loading}>
								{m.later}
							</Button>
							<Button type="submit" class="flex-1" disabled={loading}>
								{loading ? m.sending : m.sendConfirmationLink}
							</Button>
						</div>
					</form>
				{/if}
			</Card.Content>
		</Card.Root>
	</div>
{/if}
