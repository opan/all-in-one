<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Button } from '#lib/components/ui/button/index.js';
	import { Input } from '#lib/components/ui/input/index.js';
	import { Label } from '#lib/components/ui/label/index.js';
	import * as Card from '#lib/components/ui/card/index.js';
	import { safeNext } from '#lib/safe-next.js';
	import { passwordTooShort } from '#lib/password.js';
	import { afterLoginPath, getMe, setPassword } from '#lib/account-api.js';
	import { pageApp } from '#lib/use-app.svelte.js';

	// After logging in with a temporary password (RFC-003 §6.4): the session
	// can do nothing else until the user chooses a new password.
	const page$ = pageApp(page.url);

	const m = $derived(page$.m);
	const next = safeNext(page.url.searchParams.get('next'));
	const here = page.url.pathname + page.url.search;
	let checked = $state(false);
	let password = $state('');
	let confirm = $state('');
	let loading = $state(false);
	let error = $state('');

	onMount(async () => {
		const me = await getMe();
		if (me.status === 401) {
			await goto(`/login?next=${encodeURIComponent(next ?? '/home')}`, { replace: true });
			return;
		}
		if (me.ok && !me.data?.password_change_required) {
			await goto(next ?? '/home', { replace: true });
			return;
		}
		checked = true;
	});

	async function submit(event: Event) {
		event.preventDefault();
		error = '';
		if (passwordTooShort(password)) {
			error = m.passwordTooShort;
			return;
		}
		if (password !== confirm) {
			error = m.passwordsDontMatch;
			return;
		}
		loading = true;
		const res = await setPassword(password, page$.context);
		loading = false;
		if (res.ok) {
			await goto(afterLoginPath({ email_prompt: res.data?.email_prompt }, next), { replace: true });
		} else if (res.status === 401) {
			await goto(`/login?next=${encodeURIComponent(here)}`, { replace: true });
		} else {
			error = res.status === 400 && res.error ? res.error : m.setPasswordFailed;
		}
	}
</script>

<svelte:head>
	<title>{m.setPasswordTitle}</title>
</svelte:head>

{#if page$.ready && checked}
	<div class="flex items-start justify-center px-4 pt-10 sm:pt-16">
		<Card.Root class="w-full max-w-md">
			<Card.Header>
				<Card.Title class="text-2xl">{m.setPasswordTitle}</Card.Title>
				<Card.Description class="mt-2">{m.setPasswordSubtitle}</Card.Description>
			</Card.Header>
			<Card.Content>
				<form onsubmit={submit} class="space-y-4">
					{#if error}
						<div
							class="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-600 dark:border-red-800 dark:bg-red-950 dark:text-red-400"
						>
							{error}
						</div>
					{/if}
					<div class="space-y-2">
						<Label for="new-password">{m.newPassword}</Label>
						<Input
							id="new-password"
							type="password"
							autocomplete="new-password"
							bind:value={password}
							disabled={loading}
							required
						/>
					</div>
					<div class="space-y-2">
						<Label for="confirm-password">{m.confirmPassword}</Label>
						<Input
							id="confirm-password"
							type="password"
							autocomplete="new-password"
							placeholder={m.reenterPassword}
							bind:value={confirm}
							disabled={loading}
							required
						/>
					</div>
					<Button type="submit" class="w-full" disabled={loading}>
						{loading ? m.saving : m.saveAndContinue}
					</Button>
				</form>
			</Card.Content>
		</Card.Root>
	</div>
{/if}
