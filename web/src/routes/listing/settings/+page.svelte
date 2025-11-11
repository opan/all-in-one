<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Separator } from '$lib/components/ui/separator';
	import SettingsNav from "../../../components/settings-nav.svelte";
	
	let username = $state('');
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
							<div class="space-y-2">
								<Label for="username">Username</Label>
								<Input 
									id="username" 
									type="text" 
									placeholder="Enter your username" 
									bind:value={username}
									class="max-w-xl"
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
									class="max-w-xl"
								/>
							</div>

							<div class="space-y-2">
								<Label for="password-confirmation">Password Confirmation</Label>
								<Input 
									id="password-confirmation" 
									type="password" 
									placeholder="Confirm your password" 
									bind:value={passwordConfirmation}
									class="max-w-xl"
								/>
							</div>

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