<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Separator } from '$lib/components/ui/separator';
	import * as Field from '$lib/components/ui/field';
	import SettingsNav from "../../../components/settings-nav.svelte";
	
	let { data } = $props();
	
	let username = $state(data.user?.username || '');
	let email = $state(data.user?.email || '');
	let password = $state('');
	let passwordConfirmation = $state('');
	let activeSection = $state('account');

	const navItems = [
		{ id: 'account', label: 'Account', icon: '🔧' },
		{ id: 'advanced', label: 'Advanced', icon: '⚙️' }
	];

	function handleAccountSubmit(event: Event) {
		event.preventDefault();
		console.log('Account settings saved:', { username, password, passwordConfirmation });
		alert('Settings saved successfully! (Dummy action)');
	}

	function handleSectionChange(id: string) {
		activeSection = id;
	}
</script>

<div class="container mx-auto p-6">
	<div class="space-y-6">
		<!-- Header -->
		<div>
			<h1 class="text-3xl font-bold tracking-tight">Settings</h1>
			<p class="text-muted-foreground">Manage your settings and preferences</p>
		</div>

		<Separator />

		<!-- Main Content with Sidebar Navigation -->
		<div class="flex gap-8">
			<!-- Sidebar Navigation -->
			<SettingsNav 
				items={navItems} 
				activeSection={activeSection} 
				onSelect={handleSectionChange} 
			/>

			<!-- Content Area -->
			<div class="flex-1">
				{#if activeSection === 'account'}
					<div class="space-y-6">
						<div class="space-y-2">
							<h2 class="text-2xl font-semibold">Account</h2>
							<p class="text-sm text-muted-foreground">
								Update your account settings. Set your preferred username and password.
							</p>
						</div>

						<form onsubmit={handleAccountSubmit} class="space-y-6">
							<Field.Field>
								<Field.Label for="username">Username</Field.Label>
								<Input 
									id="username" 
									type="text" 
									placeholder="Enter your username" 
									bind:value={username}
								disabled
								class="max-w-xl"
							/>
							<Field.Description>
								This is your public display name.
							</Field.Description>
						</Field.Field>

						<Field.Field>
							<Field.Label for="email">Email</Field.Label>
							<Input 
								id="email" 
								type="email" 
								placeholder="Email address" 
								bind:value={email}
								disabled
								class="max-w-xl"
							/>
							</Field.Field>

							<Field.Field>
								<Field.Label for="password">Password</Field.Label>
								<Input 
									id="password" 
									type="password" 
									placeholder="Enter your password" 
									bind:value={password}
									class="max-w-xl"
								/>
							</Field.Field>

							<Field.Field>
								<Field.Label for="password-confirmation">Password Confirmation</Field.Label>
								<Input 
									id="password-confirmation" 
									type="password" 
									placeholder="Confirm your password" 
									bind:value={passwordConfirmation}
									class="max-w-xl"
								/>
							</Field.Field>

							<Button type="submit">Save Changes</Button>
						</form>
					</div>
				{:else if activeSection === 'advanced'}
					<div class="space-y-6">
						<div class="space-y-2">
							<h2 class="text-2xl font-semibold">Advanced</h2>
							<p class="text-sm text-muted-foreground">
								Configure advanced settings for your account.
							</p>
						</div>

						<div class="text-sm text-muted-foreground">
							Advanced settings will be available here.
						</div>
					</div>
				{/if}
			</div>
		</div>
	</div>
</div>