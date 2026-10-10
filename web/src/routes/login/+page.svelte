<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Card from '$lib/components/ui/card';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import type { DemoMode } from '$lib/config';
	import { safeNext } from '$lib/safe-next';
	import { authRequestIdFromNext, getAuthRequest, type AuthRequestInfo } from '$lib/oidc-api';
	import { authMessages } from '$lib/oauth-i18n';

	// Demo-account flag from the root layout load (GET /api/v1/config).
	const demo = $derived(($page.data.demoMode ?? { enabled: false }) as DemoMode);

	// Where to go after logging in: a same-site path from ?next= (e.g. the
	// OIDC hand-off at /oauth/login), otherwise the dashboard.
	const next = $derived(safeNext($page.url.searchParams.get('next')));
	const afterLogin = () => goto(next ?? '/home', { replaceState: true });

	// When an app sent the user here, this is that app's login: its name and
	// language, without aio-only extras. Wait for the app's details before
	// showing the form so its language doesn't flash in after English.
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

	// Credentials go through plain fetch, not the api.ts client: that client
	// treats any 401 as an expired session and redirects to /login, which
	// would turn a mistyped password into a lost ?next= (the app hand-off).
	function postCredentials(url: string, body: unknown): Promise<Response> {
		return fetch(url, {
			method: 'POST',
			credentials: 'include',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(body)
		});
	}
	const translated = $derived(!!app?.locale && app.locale !== 'en');

	// aio's English API messages are shown as-is; in another language the
	// page uses its own sentence for each status.
	function loginError(status: number, apiError: string | undefined): string {
		if (!translated && apiError) return apiError;
		switch (status) {
			case 401:
			case 404:
				return m.invalidCredentials;
			case 403:
				return apiError?.includes('blocked') ? m.accountBlocked : m.loginFailed;
			case 429:
				return m.tooManyAttempts;
			default:
				return m.loginFailed;
		}
	}

	let username = $state('');
	let password = $state('');
	let loading = $state(false);
	let error = $state('');

	// 2FA state
	let show2FAStep = $state(false);
	let challengeToken = $state('');
	let totpCode = $state('');
	let useRecoveryCode = $state(false);
	let recoveryCode = $state('');

	async function handleLogin(event?: Event) {
		event?.preventDefault();

		if (!username || !password) {
			error = m.credentialsRequired;
			return;
		}

		loading = true;
		error = '';

		try {
			const response = await postCredentials('/api/v1/sessions', {
				username,
				password,
			});

			const data = await response.json();

			if (response.ok && data.success) {
				if (data.data?.requires_2fa) {
					challengeToken = data.data.challenge_token;
					show2FAStep = true;
				} else {
					await afterLogin();
				}
			} else {
				error = loginError(response.status, data.error);
			}
		} catch (err) {
			console.error('Login error:', err);
			error = m.loginError;
		} finally {
			loading = false;
		}
	}

	async function handleVerify2FA(event?: Event) {
		event?.preventDefault();

		const code = useRecoveryCode ? recoveryCode.trim() : totpCode.trim();
		if (!code) {
			error = useRecoveryCode ? m.recoveryCodeRequired : m.verificationCodeRequired;
			return;
		}

		loading = true;
		error = '';

		try {
			const endpoint = useRecoveryCode
				? '/api/v1/sessions/2fa/recovery'
				: '/api/v1/sessions/2fa/verify';

			const body = useRecoveryCode
				? { challenge_token: challengeToken, recovery_code: code }
				: { challenge_token: challengeToken, code };

			const response = await postCredentials(endpoint, body);
			const data = await response.json();

			if (response.ok && data.success) {
				await afterLogin();
			} else if (response.status === 429) {
				error = m.tooManyCodeAttempts;
				reset2FA();
			} else {
				error = translated ? m.invalidCode : data.error || m.invalidCode;
			}
		} catch (err) {
			console.error('2FA verification error:', err);
			error = m.loginError;
		} finally {
			loading = false;
		}
	}

	function reset2FA() {
		show2FAStep = false;
		challengeToken = '';
		totpCode = '';
		recoveryCode = '';
		useRecoveryCode = false;
	}

	async function handleGoogleLogin() {
		// TODO: Implement Google OAuth login
		console.log('Google login attempt');
	}
</script>

<svelte:head>
	{#if app}<title>{m.loginTitleApp(app.client_name)}</title>{/if}
</svelte:head>

{#if appLookupDone}
<div class="flex items-start justify-center px-4 {appRequestId ? 'pt-10 sm:pt-16' : 'pt-[33vh]'}">
	<Card.Root class="w-full max-w-md">
		<Card.Header>
			<div class="flex justify-between items-start">
				<div>
					{#if show2FAStep}
						<Card.Title class="text-2xl">{m.twoFactorTitle}</Card.Title>
						<Card.Description class="mt-2">
							{useRecoveryCode ? m.enterRecoveryCode : m.enterTotpCode}
						</Card.Description>
					{:else}
						<Card.Title class="text-2xl">{app ? m.loginTitleApp(app.client_name) : m.loginTitle}</Card.Title>
						<Card.Description class="mt-2">
							{app ? m.loginSubtitleApp : m.loginSubtitle}
						</Card.Description>
					{/if}
				</div>
				{#if !show2FAStep && !app}
					<Button variant="ghost" class="text-sm" onclick={() => goto(next ? `/signup?next=${encodeURIComponent(next)}` : '/signup')}>{m.signUp}</Button>
				{/if}
			</div>
		</Card.Header>
		<Card.Content class="space-y-4">
			{#if error}
				<div class="p-3 text-sm text-red-600 bg-red-50 border border-red-200 rounded-md dark:bg-red-950 dark:border-red-800 dark:text-red-400">
					{error}
				</div>
			{/if}

			{#if show2FAStep}
				<form onsubmit={handleVerify2FA} class="space-y-4">
					{#if useRecoveryCode}
						<div class="space-y-2">
							<Label for="recovery-code">{m.recoveryCode}</Label>
							<Input
								id="recovery-code"
								type="text"
								placeholder="XXXX-XXXX"
								bind:value={recoveryCode}
								disabled={loading}
								required
								class="font-mono tracking-wider"
							/>
						</div>
					{:else}
						<div class="space-y-2">
							<Label for="totp-code">{m.verificationCode}</Label>
							<Input
								id="totp-code"
								type="text"
								inputmode="numeric"
								maxlength={6}
								placeholder="000000"
								bind:value={totpCode}
								disabled={loading}
								required
								class="font-mono text-center text-2xl tracking-[0.5em]"
							/>
						</div>
					{/if}
					<Button type="submit" class="w-full" disabled={loading}>
						{loading ? m.verifying : m.verify}
					</Button>
				</form>

				<div class="flex justify-between items-center text-sm">
					<button
						type="button"
						class="text-muted-foreground hover:underline"
						onclick={() => { useRecoveryCode = !useRecoveryCode; error = ''; }}
					>
						{useRecoveryCode ? m.useAuthenticator : m.useRecoveryCode}
					</button>
					<button
						type="button"
						class="text-muted-foreground hover:underline"
						onclick={reset2FA}
					>
						{m.backToLogin}
					</button>
				</div>
			{:else}
				<form onsubmit={handleLogin} class="space-y-4">
					<div class="space-y-2">
						<Label for="username">{m.username}</Label>
						<Input
							id="username"
							type="text"
							placeholder={m.usernamePlaceholder}
							bind:value={username}
							disabled={loading}
							required
						/>
					</div>
					<div class="space-y-2">
						<div class="flex justify-between items-center">
							<Label for="password">{m.password}</Label>
							{#if !app}
								<a href="/forgot-password" class="text-sm text-muted-foreground hover:underline">
									Forgot your password?
								</a>
							{/if}
						</div>
						<Input
							id="password"
							type="password"
							bind:value={password}
							disabled={loading}
							required
							placeholder={m.passwordPlaceholder}
						/>
					</div>
					<Button type="submit" class="w-full" disabled={loading}>
						{loading ? m.loggingIn : m.loginButton}
					</Button>
				</form>
				{#if app}
					<p class="text-sm text-center text-muted-foreground">
						{m.noAccount}
						<a href={`/signup?next=${encodeURIComponent(next ?? '')}`} class="underline hover:text-foreground">{m.signUp}</a>
					</p>
				{:else}
					<Button variant="outline" class="w-full" onclick={() => handleGoogleLogin()} disabled={loading}>
						Login with Google
					</Button>
				{/if}

				{#if demo.enabled && !authRequestIdFromNext(next)}
					<div class="rounded-md border border-primary/20 bg-primary/5 p-3 text-sm">
						<div class="flex items-center justify-between gap-2">
							<span class="font-medium">Just want to look around?</span>
							<button
								type="button"
								class="text-primary hover:underline disabled:opacity-50"
								disabled={loading}
								onclick={() => {
									username = demo.username ?? '';
									password = demo.password ?? '';
									error = '';
								}}
							>
								Use demo account
							</button>
						</div>
						<p class="mt-1 text-muted-foreground">
							No sign-up needed — log in with
							<code class="font-mono font-semibold text-foreground">{demo.username}</code>
							/
							<code class="font-mono font-semibold text-foreground">{demo.password}</code>.
						</p>
					</div>
				{/if}
			{/if}
		</Card.Content>
	</Card.Root>
</div>
{/if}
