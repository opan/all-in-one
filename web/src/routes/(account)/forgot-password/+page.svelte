<script lang="ts">
	import { page } from '$app/stores';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Card from '$lib/components/ui/card';
	import { safeNext } from '$lib/safe-next';
	import { forgotPassword } from '$lib/account-api';
	import { pageApp } from '$lib/use-app.svelte';
	import type { PublicConfig } from '$lib/config';

	// Forgot password (RFC-003 §6.4, §6.5): users with a confirmed email get a
	// temporary password; everyone else is told how to reach the admin.
	const page$ = pageApp($page.url);
	const m = $derived(page$.m);
	const config = $derived($page.data.publicConfig as PublicConfig | undefined);
	const emailEnabled = $derived(config?.email_enabled ?? false);
	const support = $derived(config?.support_email ?? '');

	const next = safeNext($page.url.searchParams.get('next'));
	const back = $derived.by(() => {
		if (next) return { href: `/login?next=${encodeURIComponent(next)}`, label: m.backToLogin };
		if (page$.app?.return_to) return { href: page$.app.return_to, label: m.backTo(page$.app.client_name) };
		return { href: '/login', label: m.backToLogin };
	});

	let login = $state('');
	let loading = $state(false);
	let sent = $state(false);
	let error = $state('');

	const mailto = $derived(
		`mailto:${support}?subject=${encodeURIComponent(m.mailtoSubject)}&body=${encodeURIComponent(
			m.mailtoBody(login.includes('@') ? '' : login.trim())
		)}`
	);

	async function submit(event: Event) {
		event.preventDefault();
		if (!login.trim()) return;
		loading = true;
		error = '';
		const res = await forgotPassword(login.trim(), page$.context);
		loading = false;
		if (res.ok) sent = true;
		else if (res.status === 429) error = m.tooManyAttempts;
		else if (res.status === 503) error = m.emailResetUnavailable;
		else error = m.somethingWrong;
	}
</script>

<svelte:head>
	<title>{m.forgotTitle}</title>
</svelte:head>

{#if page$.ready}
	<div class="flex items-start justify-center px-4 pt-10 sm:pt-16">
		<Card.Root class="w-full max-w-md">
			<Card.Header>
				<Card.Title class="text-2xl">{sent ? m.forgotSent : m.forgotTitle}</Card.Title>
				<Card.Description class="mt-2">
					{#if sent}
						{m.forgotSentDetail}
					{:else if emailEnabled}
						{m.forgotSubtitle}
					{:else}
						{m.emailResetUnavailable}
					{/if}
				</Card.Description>
			</Card.Header>
			<Card.Content class="space-y-5">
				{#if error}
					<div
						class="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-600 dark:border-red-800 dark:bg-red-950 dark:text-red-400"
					>
						{error}
					</div>
				{/if}

				{#if emailEnabled && !sent}
					<form onsubmit={submit} class="space-y-4">
						<div class="space-y-2">
							<Label for="login">{m.usernameOrEmail}</Label>
							<Input
								id="login"
								autocomplete="username"
								placeholder={m.usernameOrEmailPlaceholder}
								bind:value={login}
								disabled={loading}
								required
							/>
						</div>
						<Button type="submit" class="w-full" disabled={loading}>
							{loading ? m.sending : m.sendTempPassword}
						</Button>
					</form>
				{/if}

				<div class="rounded-md border bg-muted/40 p-3 text-sm" data-testid="no-email-help">
					<p class="font-medium">{m.noEmailTitle}</p>
					{#if support}
						<p class="mt-1 text-muted-foreground">{m.noEmailText(support)}</p>
						<a href={mailto} class="mt-2 inline-block text-primary underline">{m.emailAdmin}</a>
					{:else}
						<p class="mt-1 text-muted-foreground">{m.noEmailNoSupport}</p>
					{/if}
				</div>

				<p class="text-center text-sm">
					<a href={back.href} class="text-muted-foreground hover:underline">{back.label}</a>
				</p>
			</Card.Content>
		</Card.Root>
	</div>
{/if}
