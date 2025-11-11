<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Tabs from '$lib/components/ui/tabs';
	
	let username = $state('');
	let password = $state('');
	let passwordConfirmation = $state('');

	function handleAccountSubmit(event: Event) {
		event.preventDefault();
		// Dummy action for now
		console.log('Account settings saved:', { username, password, passwordConfirmation });
		alert('Settings saved successfully! (Dummy action)');
	}
</script>

<div class="container mx-auto p-6">
	<div class="space-y-6">
		<div>
			<h1 class="text-3xl font-bold tracking-tight">Settings</h1>
			<p class="text-muted-foreground">Manage your setting and preferences</p>
		</div>

		<Tabs.Root value="account" class="w-full">
			<Tabs.List>
				<Tabs.Trigger value="account">Account</Tabs.Trigger>
				<Tabs.Trigger value="advanced">Advanced</Tabs.Trigger>
			</Tabs.List>

			<Tabs.Content value="account" class="space-y-4">
				<div class="space-y-2">
					<h2 class="text-xl font-semibold">Account</h2>
					<p class="text-sm text-muted-foreground">
						Update your account settings. Set your preferred username and password.
					</p>
				</div>

				<form onsubmit={handleAccountSubmit} class="space-y-4">
					<div class="space-y-2">
						<Label for="username">Username</Label>
						<Input 
							id="username" 
							type="text" 
							placeholder="Enter your username" 
							bind:value={username}
							class="max-w-md"
						/>
						<p class="text-xs text-muted-foreground">
							This is your public display name.
						</p>
					</div>

					<div class="space-y-2">
						<Label for="password">Password</Label>
						<Input 
							id="password" 
							type="password" 
							placeholder="Enter your password" 
							bind:value={password}
							class="max-w-md"
						/>
					</div>

					<div class="space-y-2">
						<Label for="password-confirmation">Password Confirmation</Label>
						<Input 
							id="password-confirmation" 
							type="password" 
							placeholder="Confirm your password" 
							bind:value={passwordConfirmation}
							class="max-w-md"
						/>
					</div>

					<Button type="submit">Save Changes</Button>
				</form>
			</Tabs.Content>

			<Tabs.Content value="advanced" class="space-y-4">
				<div class="space-y-2">
					<h2 class="text-xl font-semibold">Advanced</h2>
					<p class="text-sm text-muted-foreground">
						Configure advanced settings for your account.
					</p>
				</div>

				<div class="text-sm text-muted-foreground">
					Advanced settings will be available here.
				</div>
			</Tabs.Content>
		</Tabs.Root>
	</div>
</div>