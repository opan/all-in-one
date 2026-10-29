<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Button } from '#lib/components/ui/button/index.js';
	import { Input } from '#lib/components/ui/input/index.js';
	import { Label } from '#lib/components/ui/label/index.js';
	import * as Card from '#lib/components/ui/card/index.js';
	import { passwordTooShort } from '#lib/password.js';
	import { changePassword, getMe, updateMyEmail, type Me } from '#lib/account-api.js';
	import { pageApp } from '#lib/use-app.svelte.js';
	import type { PublicConfig } from '#lib/config.js';

	// Password and email for users who come from an app (RFC-003 §6.7):
	// /account?client_id=cashflow&return_to=…&ui_locales=id. Presented as the
	// app, so they never see aio's own dashboard.
	const page$ = pageApp(page.url);

	const m = $derived(page$.m);
	const emailEnabled = $derived((page.data.publicConfig as PublicConfig | undefined)?.email_enabled ?? false);
	const here = page.url.pathname + page.url.search;
	let me = $state<Me | null>(null);
	let newEmail = $state('');
	let emailBusy = $state(false);
	let emailError = $state('');
	let emailNote = $state('');

	let currentPassword = $state('');
	let password = $state('');
	let confirm = $state('');
	let passwordBusy = $state(false);
	let passwordError = $state('');
	let passwordNote = $state('');

	onMount(async () => {
		const res = await getMe();
		if (res.status === 401) {
			await goto(`/login?next=${encodeURIComponent(here)}`, { replace: true });
			return;
		}
		me = res.data;
	});

	async function saveEmail(event: Event) {
		event.preventDefault();
		emailError = '';
		emailNote = '';
		emailBusy = true;
		const res = await updateMyEmail(newEmail.trim(), page$.context);
		emailBusy = false;
		if (res.ok && me) {
			me = { ...me, pending_email: res.data?.pending_email };
			emailNote = m.confirmationSent(res.data?.pending_email ?? newEmail.trim());
			newEmail = '';
			return;
		}
		emailError =
			res.status === 400
				? res.error === 'this is already your email'
					? res.error
					: m.invalidEmail
				: res.status === 409
					? m.emailCantBeUsed
					: res.status === 429
						? m.tooManyEmails
						: m.emailNotSent;
	}

	async function savePassword(event: Event) {
		event.preventDefault();
		passwordError = '';
		passwordNote = '';
		if (passwordTooShort(password)) {
			passwordError = m.passwordTooShort;
			return;
		}
		if (password !== confirm) {
			passwordError = m.passwordsDontMatch;
			return;
		}
		passwordBusy = true;
		const res = await changePassword(currentPassword, password);
		passwordBusy = false;
		if (res.ok) {
			passwordNote = m.passwordChanged;
			currentPassword = password = confirm = '';
		} else {
			passwordError = res.status === 401 ? m.wrongCurrentPassword : m.setPasswordFailed;
		}
	}
</script>

<svelte:head>
	<title>{m.accountTitle}</title>
</svelte:head>

{#snippet alert(text: string, kind: 'error' | 'ok')}
	<div
		class="rounded-md border p-3 text-sm {kind === 'error'
			? 'border-red-200 bg-red-50 text-red-600 dark:border-red-800 dark:bg-red-950 dark:text-red-400'
			: 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-300'}"
	>
		{text}
	</div>
{/snippet}

{#if page$.ready && me}
	<div class="mx-auto flex max-w-md flex-col gap-4 px-4 pt-8 pb-4 sm:pt-12">
		<div>
			<h1 class="text-2xl font-semibold">{m.accountTitle}</h1>
			<p class="text-sm text-muted-foreground">
				{page$.app ? m.accountSubtitle(page$.app.client_name) : me.username}
			</p>
		</div>

		<Card.Root>
			<Card.Header>
				<Card.Title>{m.emailSection}</Card.Title>
				<Card.Description>
					{#if me.email}
						<span class="font-medium text-foreground">{me.email}</span>
						· {me.email_verified ? m.emailVerified : m.emailNotVerified}
					{:else}
						{m.noEmailYet}
					{/if}
				</Card.Description>
			</Card.Header>
			<Card.Content class="space-y-3">
				{#if me.pending_email && !emailNote}
					<p class="text-sm text-muted-foreground">{m.emailPendingNote(me.pending_email)}</p>
				{/if}
				{#if emailError}{@render alert(emailError, 'error')}{/if}
				{#if emailNote}{@render alert(emailNote, 'ok')}{/if}
				{#if emailEnabled}
					<form onsubmit={saveEmail} class="space-y-3">
						<Label for="new-email" class="sr-only">{m.email}</Label>
						<Input
							id="new-email"
							type="email"
							autocomplete="email"
							placeholder={m.emailPlaceholder}
							bind:value={newEmail}
							disabled={emailBusy}
							required
						/>
						<Button type="submit" variant="outline" class="w-full" disabled={emailBusy}>
							{emailBusy ? m.sending : m.changeEmail}
						</Button>
					</form>
				{:else}
					<p class="text-sm text-muted-foreground">{m.emailUnavailableHere}</p>
				{/if}
			</Card.Content>
		</Card.Root>

		<Card.Root>
			<Card.Header>
				<Card.Title>{m.passwordSection}</Card.Title>
			</Card.Header>
			<Card.Content>
				<form onsubmit={savePassword} class="space-y-3">
					{#if passwordError}{@render alert(passwordError, 'error')}{/if}
					{#if passwordNote}{@render alert(passwordNote, 'ok')}{/if}
					<div class="space-y-2">
						<Label for="current-password">{m.currentPassword}</Label>
						<Input
							id="current-password"
							type="password"
							autocomplete="current-password"
							bind:value={currentPassword}
							disabled={passwordBusy}
							required
						/>
					</div>
					<div class="space-y-2">
						<Label for="new-password">{m.newPassword}</Label>
						<Input
							id="new-password"
							type="password"
							autocomplete="new-password"
							bind:value={password}
							disabled={passwordBusy}
							required
						/>
					</div>
					<div class="space-y-2">
						<Label for="confirm-password">{m.confirmPassword}</Label>
						<Input
							id="confirm-password"
							type="password"
							autocomplete="new-password"
							bind:value={confirm}
							disabled={passwordBusy}
							required
						/>
					</div>
					<Button type="submit" class="w-full" disabled={passwordBusy}>
						{passwordBusy ? m.saving : m.changePassword}
					</Button>
				</form>
			</Card.Content>
		</Card.Root>

		{#if page$.app?.return_to}
			<p class="text-center text-sm">
				<a href={page$.app.return_to} class="text-muted-foreground hover:underline">
					{m.backTo(page$.app.client_name)}
				</a>
			</p>
		{/if}
	</div>
{/if}
