<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Card from '$lib/components/ui/card';
	import { goto } from '$app/navigation';
	import { apiPost } from '$lib/api';

	let username = $state('');
	let email = $state('');
	let password = $state('');
	let confirmPassword = $state('');
	let loading = $state(false);
	let error = $state('');

	async function handleRegister(event?: Event) {
		event?.preventDefault();

		if (!username || !password || !confirmPassword) {
			error = 'Username and password are required';
			return;
		}

		if (password !== confirmPassword) {
			error = 'Passwords do not match';
			return;
		}

		if (password.length < 3) {
			error = 'Password must be at least 3 characters long';
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
					error = 'Username already taken';
				} else if (response.status === 429) {
					error = 'Too many sign-up attempts. Please try again later.';
				} else {
					error = data.error || 'Failed to create account. Please try again.';
				}
				return;
			}

			// Account created — log the user in right away instead of bouncing
			// them to /login to retype what they just entered.
			const loginResponse = await apiPost('/api/v1/sessions', { username, password });
			const loginData = await loginResponse.json();

			if (loginResponse.ok && loginData.success && !loginData.data?.requires_2fa) {
				await goto('/home');
			} else {
				// Account exists even if auto-login didn't pan out — let them log in manually.
				await goto('/login');
			}
		} catch (err) {
			console.error('Registration error:', err);
			error = 'An error occurred during sign up. Please try again.';
		} finally {
			loading = false;
		}
	}
</script>

<div class="flex items-start justify-center pt-[33vh]">
	<Card.Root class="w-full max-w-md">
		<Card.Header>
			<div class="flex justify-between items-start">
				<div>
					<Card.Title class="text-2xl">Create your account</Card.Title>
					<Card.Description class="mt-2">
						Enter a username and password to get started
					</Card.Description>
				</div>
				<Button variant="ghost" class="text-sm" onclick={() => goto('/login')}>Log In</Button>
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
					<Label for="username">Username</Label>
					<Input
						id="username"
						type="text"
						placeholder="Choose a username"
						bind:value={username}
						disabled={loading}
						required
					/>
				</div>
				<div class="space-y-2">
					<Label for="email">Email <span class="text-muted-foreground">(optional)</span></Label>
					<Input
						id="email"
						type="email"
						placeholder="you@example.com"
						bind:value={email}
						disabled={loading}
					/>
				</div>
				<div class="space-y-2">
					<Label for="password">Password</Label>
					<Input
						id="password"
						type="password"
						bind:value={password}
						disabled={loading}
						required
						placeholder="Choose a password"
					/>
				</div>
				<div class="space-y-2">
					<Label for="confirm-password">Confirm Password</Label>
					<Input
						id="confirm-password"
						type="password"
						bind:value={confirmPassword}
						disabled={loading}
						required
						placeholder="Re-enter your password"
					/>
				</div>
				<Button type="submit" class="w-full" disabled={loading}>
					{loading ? 'Creating account...' : 'Sign Up'}
				</Button>
			</form>

			<p class="text-sm text-center text-muted-foreground">
				Already have an account?
				<a href="/login" class="underline hover:text-foreground">Log in</a>
			</p>
		</Card.Content>
	</Card.Root>
</div>
