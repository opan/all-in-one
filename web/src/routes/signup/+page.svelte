<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Card from '$lib/components/ui/card';
	import { goto } from '$app/navigation';
	import { apiPost } from '$lib/api';
	import { page } from '$app/stores';
	import { safeNext } from '$lib/safe-next';
	import { authRequestIdFromNext, getAuthRequest, type AuthRequestInfo } from '$lib/oidc-api';
	import { authMessages } from '$lib/oauth-i18n';

	// Same ?next= handling as the login page (see routes/login/+page.svelte).
	const next = $derived(safeNext($page.url.searchParams.get('next')));
	const loginHref = $derived(next ? `/login?next=${encodeURIComponent(next)}` : '/login');
	// As on the login page: an app's signup shows its name and language.
	const appRequestId = $derived(authRequestIdFromNext(next));
	let app = $state<AuthRequestInfo | null>(null);
	let appLookupDone = $state(false);
	$effect(() => {
		const id = appRequestId;
		app = null;
		appLookupDone = !id;
		if (id)
			getAuthRequest(id)
				.then((info) => (app = info))
				.catch(() => {})
				.finally(() => (appLookupDone = true));
	});
	const m = $derived(authMessages(app?.locale));
	const translated = $derived(!!app?.locale && app.locale !== 'en');

	let username = $state('');
	let email = $state('');
	let password = $state('');
	let confirmPassword = $state('');
	let loading = $state(false);
	let error = $state('');

	async function handleRegister(event?: Event) {
		event?.preventDefault();

		if (!username || !password || !confirmPassword) {
			error = m.usernamePasswordRequired;
			return;
		}

		if (password !== confirmPassword) {
			error = m.passwordsDontMatch;
			return;
		}

		if (password.length < 3) {
			error = m.passwordTooShort;
			return;
		}

		loading = true;
		error = '';

		try {
			const response = await apiPost('/api/v1/users', {
				username,
				password,
				email,
			});

			const data = await response.json();

			if (!response.ok || !data.success) {
				if (response.status === 409) {
					error = m.usernameTaken;
				} else if (response.status === 429) {
					error = m.tooManySignups;
				} else if (response.status === 400 && translated) {
					error = m.usernamePasswordRequired;
				} else {
					error = (!translated && data.error) || m.signupFailed;
				}
				return;
			}

			// Account created — log the user in right away instead of bouncing
			// them to /login to retype what they just entered.
			const loginResponse = await apiPost('/api/v1/sessions', { username, password });
			const loginData = await loginResponse.json();

			if (loginResponse.ok && loginData.success && !loginData.data?.requires_2fa) {
				await goto(next ?? '/home', { replaceState: true });
			} else {
				// Account exists even if auto-login didn't pan out — let them log in manually.
				await goto(loginHref);
			}
		} catch (err) {
			console.error('Registration error:', err);
			error = m.signupError;
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	{#if app}<title>{m.signupTitleApp(app.client_name)}</title>{/if}
</svelte:head>

{#if appLookupDone}
<div class="flex items-start justify-center px-4 {appRequestId ? 'pt-10 sm:pt-16' : 'pt-[33vh]'}">
	<Card.Root class="w-full max-w-md">
		<Card.Header>
			<div class="flex justify-between items-start">
				<div>
					<Card.Title class="text-2xl">{app ? m.signupTitleApp(app.client_name) : m.signupTitle}</Card.Title>
					<Card.Description class="mt-2">
						{app ? m.signupSubtitleApp(app.client_name) : m.signupSubtitle}
					</Card.Description>
				</div>
				{#if !app}
					<Button variant="ghost" class="text-sm" onclick={() => goto(loginHref)}>{m.logIn}</Button>
				{/if}
			</div>
		</Card.Header>
		<Card.Content class="space-y-4">
			{#if error}
				<div class="p-3 text-sm text-red-600 bg-red-50 border border-red-200 rounded-md dark:bg-red-950 dark:border-red-800 dark:text-red-400">
					{error}
				</div>
			{/if}

			<form onsubmit={handleRegister} class="space-y-4">
				<div class="space-y-2">
					<Label for="username">{m.username}</Label>
					<Input
						id="username"
						type="text"
						placeholder={m.chooseUsername}
						bind:value={username}
						disabled={loading}
						required
					/>
				</div>
				<div class="space-y-2">
					<Label for="email">{m.email} <span class="text-muted-foreground">{m.optional}</span></Label>
					<Input
						id="email"
						type="email"
						placeholder="you@example.com"
						bind:value={email}
						disabled={loading}
					/>
				</div>
				<div class="space-y-2">
					<Label for="password">{m.password}</Label>
					<Input
						id="password"
						type="password"
						bind:value={password}
						disabled={loading}
						required
						placeholder={m.choosePassword}
					/>
				</div>
				<div class="space-y-2">
					<Label for="confirm-password">{m.confirmPassword}</Label>
					<Input
						id="confirm-password"
						type="password"
						bind:value={confirmPassword}
						disabled={loading}
						required
						placeholder={m.reenterPassword}
					/>
				</div>
				<Button type="submit" class="w-full" disabled={loading}>
					{loading ? m.creatingAccount : m.signUpButton}
				</Button>
			</form>

			<p class="text-sm text-center text-muted-foreground">
				{m.haveAccount}
				<a href={loginHref} class="underline hover:text-foreground">{m.logIn}</a>
			</p>
		</Card.Content>
	</Card.Root>
</div>
{/if}
