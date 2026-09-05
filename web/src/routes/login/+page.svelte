<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Card from '$lib/components/ui/card';
	import { goto } from '$app/navigation';
	import { apiPost } from '$lib/api';
	import { page } from '$app/stores';
	import type { DemoMode } from '$lib/config';

	// Demo-account flag from the root layout load (GET /api/v1/config).
	const demo = $derived(($page.data.demoMode ?? { enabled: false }) as DemoMode);

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
			error = 'Username and password are required';
			return;
		}

		loading = true;
		error = '';

		try {
			const response = await apiPost('/api/v1/sessions', {
				username,
				password,
			});

			const data = await response.json();

			if (response.ok && data.success) {
				if (data.data?.requires_2fa) {
					challengeToken = data.data.challenge_token;
					show2FAStep = true;
				} else {
					await goto('/home');
				}
			} else {
				error = data.error || 'Login failed. Please check your credentials.';
			}
		} catch (err) {
			console.error('Login error:', err);
			error = 'An error occurred during login. Please try again.';
		} finally {
			loading = false;
		}
	}

	async function handleVerify2FA(event?: Event) {
		event?.preventDefault();

		const code = useRecoveryCode ? recoveryCode.trim() : totpCode.trim();
		if (!code) {
			error = useRecoveryCode ? 'Recovery code is required' : 'Verification code is required';
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

			const response = await apiPost(endpoint, body);
			const data = await response.json();

			if (response.ok && data.success) {
				await goto('/home');
			} else if (response.status === 429) {
				error = 'Too many attempts. Please login again.';
				reset2FA();
			} else {
				error = data.error || 'Invalid code. Please try again.';
			}
		} catch (err) {
			console.error('2FA verification error:', err);
			error = 'An error occurred. Please try again.';
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

<div class="flex items-start justify-center pt-[33vh]">
	<Card.Root class="w-full max-w-md">
		<Card.Header>
			<div class="flex justify-between items-start">
				<div>
					{#if show2FAStep}
						<Card.Title class="text-2xl">Two-Factor Authentication</Card.Title>
						<Card.Description class="mt-2">
							{#if useRecoveryCode}
								Enter one of your recovery codes
							{:else}
								Enter the 6-digit code from your authenticator app
							{/if}
						</Card.Description>
					{:else}
						<Card.Title class="text-2xl">Login to your account</Card.Title>
						<Card.Description class="mt-2">
							Enter your email below to login to your account
						</Card.Description>
					{/if}
				</div>
				{#if !show2FAStep}
					<Button variant="ghost" class="text-sm" onclick={() => goto('/signup')}>Sign Up</Button>
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
							<Label for="recovery-code">Recovery Code</Label>
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
							<Label for="totp-code">Verification Code</Label>
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
						{loading ? 'Verifying...' : 'Verify'}
					</Button>
				</form>

				<div class="flex justify-between items-center text-sm">
					<button
						type="button"
						class="text-muted-foreground hover:underline"
						onclick={() => { useRecoveryCode = !useRecoveryCode; error = ''; }}
					>
						{useRecoveryCode ? 'Use authenticator app instead' : 'Use a recovery code'}
					</button>
					<button
						type="button"
						class="text-muted-foreground hover:underline"
						onclick={reset2FA}
					>
						Back to login
					</button>
				</div>
			{:else}
				<form onsubmit={handleLogin} class="space-y-4">
					<div class="space-y-2">
						<Label for="username">Username</Label>
						<Input
							id="username"
							type="text"
							placeholder="Enter your username"
							bind:value={username}
							disabled={loading}
							required
						/>
					</div>
					<div class="space-y-2">
						<div class="flex justify-between items-center">
							<Label for="password">Password</Label>
							<a href="/forgot-password" class="text-sm text-muted-foreground hover:underline">
								Forgot your password?
							</a>
						</div>
						<Input
							id="password"
							type="password"
							bind:value={password}
							disabled={loading}
							required
							placeholder="Your password"
						/>
					</div>
					<Button type="submit" class="w-full" disabled={loading}>
						{loading ? 'Logging in...' : 'Login'}
					</Button>
				</form>
				<Button variant="outline" class="w-full" onclick={() => handleGoogleLogin()} disabled={loading}>
					Login with Google
				</Button>

				{#if demo.enabled}
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
