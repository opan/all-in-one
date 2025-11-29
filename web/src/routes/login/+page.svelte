<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Card from '$lib/components/ui/card';
	import { goto } from '$app/navigation';
	import { apiPost } from '$lib/api';

	let username = $state('');
	let password = $state('');
	let loading = $state(false);
	let error = $state('');

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
				// Redirect to main page on success
				await goto('/');
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
					<Card.Title class="text-2xl">Login to your account</Card.Title>
					<Card.Description class="mt-2">
						Enter your email below to login to your account
					</Card.Description>
				</div>
				<Button variant="ghost" class="text-sm">Sign Up</Button>
			</div>
		</Card.Header>
		<Card.Content class="space-y-4">
			{#if error}
				<div class="p-3 text-sm text-red-600 bg-red-50 border border-red-200 rounded-md">
					{error}
				</div>
			{/if}
			
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
						placeholder={"Your password"}
					/>
				</div>
				<Button type="submit" class="w-full" disabled={loading}>
					{loading ? 'Logging in...' : 'Login'}
				</Button>
			</form>
			<Button variant="outline" class="w-full" onclick={() => handleGoogleLogin()} disabled={loading}>
				Login with Google
			</Button>
		</Card.Content>
	</Card.Root>
</div>
