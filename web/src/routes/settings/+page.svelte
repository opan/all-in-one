<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Separator } from '$lib/components/ui/separator';
	import * as Field from '$lib/components/ui/field';
	import * as Dialog from '$lib/components/ui/dialog';
	import SettingsNav from "../../components/settings-nav.svelte";
	import { toast, Toaster } from 'svelte-sonner';
	import { apiGet, apiPost, apiClient } from '$lib/api';
	import { goto } from '$app/navigation';
	import { setMode, resetMode, userPrefersMode } from 'mode-watcher';
	import { PALETTES, palette, setPalette } from '$lib/stores/theme';

	let { data } = $props();

	let username = $state(data.user?.username || '');
	let email = $state(data.user?.email || '');
	let currentPassword = $state('');
	let password = $state('');
	let passwordConfirmation = $state('');
	let activeSection = $state('general');
	let mobileShowNav = $state(true);

	const navItems = [
		{ id: 'general', label: 'General', icon: '⚙️' },
		{ id: 'account', label: 'Account', icon: '👤' },
		{ id: 'advanced', label: 'Advanced', icon: '🛠️' }
	];

	// 2FA state
	let totpStatus: { enabled: boolean; recovery_codes_remaining: number } | null = $state(null);
	let totpLoading = $state(false);

	// Setup flow state
	let setupDialogOpen = $state(false);
	let setupStep: 'qr' | 'verify' | 'recovery' = $state('qr');
	let setupData: { qr_code: string; secret: string; recovery_codes: string[] } | null = $state(null);
	let setupVerifyCode = $state('');

	// Disable flow state
	let disableDialogOpen = $state(false);
	let disablePassword = $state('');

	// Regenerate recovery codes state
	let regenDialogOpen = $state(false);
	let regenPassword = $state('');
	let regenCodes: string[] | null = $state(null);

	async function loadTOTPStatus() {
		try {
			const response = await apiGet('/api/v1/users/2fa/status');
			if (response.ok) {
				const result = await response.json();
				totpStatus = result.data;
			}
		} catch (err) {
			console.error('Failed to load 2FA status:', err);
		}
	}

	// Load 2FA status when Advanced tab is selected
	$effect(() => {
		if (activeSection === 'advanced' && totpStatus === null) {
			loadTOTPStatus();
		}
	});

	async function handleSetupTOTP() {
		totpLoading = true;
		try {
			const response = await apiPost('/api/v1/users/2fa/setup');
			const result = await response.json();
			if (response.ok && result.success) {
				setupData = result.data;
				setupStep = 'qr';
				setupDialogOpen = true;
			} else {
				toast.error(result.error || 'Failed to start 2FA setup');
			}
		} catch (err) {
			toast.error('Failed to start 2FA setup');
		} finally {
			totpLoading = false;
		}
	}

	async function handleVerifySetup() {
		if (!setupVerifyCode || setupVerifyCode.length !== 6) {
			toast.error('Please enter a 6-digit code');
			return;
		}
		totpLoading = true;
		try {
			const response = await apiPost('/api/v1/users/2fa/verify-setup', {
				code: setupVerifyCode
			});
			const result = await response.json();
			if (response.ok && result.success) {
				setupStep = 'recovery';
				toast.success('2FA has been enabled!');
			} else {
				toast.error(result.error || 'Invalid code');
			}
		} catch (err) {
			toast.error('Verification failed');
		} finally {
			totpLoading = false;
		}
	}

	function handleSetupComplete() {
		setupDialogOpen = false;
		setupData = null;
		setupVerifyCode = '';
		setupStep = 'qr';
		// Session was invalidated, redirect to login
		goto('/login');
	}

	async function handleDisableTOTP() {
		if (!disablePassword) {
			toast.error('Password is required');
			return;
		}
		totpLoading = true;
		try {
			const response = await apiClient('/api/v1/users/2fa', {
				method: 'DELETE',
				body: JSON.stringify({ current_password: disablePassword })
			});
			const result = await response.json();
			if (response.ok && result.success) {
				toast.success('2FA has been disabled');
				disableDialogOpen = false;
				disablePassword = '';
				totpStatus = { enabled: false, recovery_codes_remaining: 0 };
			} else {
				toast.error(result.error || 'Failed to disable 2FA');
			}
		} catch (err) {
			toast.error('Failed to disable 2FA');
		} finally {
			totpLoading = false;
		}
	}

	async function handleRegenerateRecoveryCodes() {
		if (!regenPassword) {
			toast.error('Password is required');
			return;
		}
		totpLoading = true;
		try {
			const response = await apiPost('/api/v1/users/2fa/recovery-codes/regenerate', {
				current_password: regenPassword
			});
			const result = await response.json();
			if (response.ok && result.success) {
				regenCodes = result.data.recovery_codes;
				toast.success('Recovery codes regenerated');
			} else {
				toast.error(result.error || 'Failed to regenerate codes');
			}
		} catch (err) {
			toast.error('Failed to regenerate codes');
		} finally {
			totpLoading = false;
		}
	}

	function handleRegenComplete() {
		regenDialogOpen = false;
		regenPassword = '';
		regenCodes = null;
		loadTOTPStatus();
	}

	function downloadRecoveryCodes(codes: string[]) {
		const text = `All-In-One Recovery Codes\nGenerated: ${new Date().toISOString()}\n\nKeep these codes in a safe place. Each code can only be used once.\n\n${codes.map((c, i) => `${i + 1}. ${c}`).join('\n')}\n`;
		const blob = new Blob([text], { type: 'text/plain' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = 'all-in-one-recovery-codes.txt';
		a.click();
		URL.revokeObjectURL(url);
	}

	function handleAccountSubmit(event: Event) {
		event.preventDefault();

		if (!password || !passwordConfirmation || !currentPassword) {
			toast.error('Please fill all required fields');
			return;
		}

		if (password !== passwordConfirmation) {
			toast.error('Passwords do not match');
			return;
		}

		if (password.length < 3) {
			toast.error('Password must be at least 3 characters long');
			return;
		}

		fetch('/api/v1/users/reset_password', {
			method: 'POST',
			headers: {
				'Content-Type': 'application/json',
			},
			body: JSON.stringify({
				username,
				current_password: currentPassword,
				password,
				password_confirmation: passwordConfirmation
			})
		})
		.then(async (res) => {
			const result = await res.json();
			if (res.ok && result.success) {
				toast.success('Password reset successfully!');
				currentPassword = '';
				password = '';
				passwordConfirmation = '';
			} else {
				toast.error(result.error || 'Failed to reset password.');
			}
		})
		.catch(() => {
			toast.error('Network error. Please try again.');
		});
	}

	function handleSectionChange(id: string) {
		activeSection = id;
		mobileShowNav = false;
	}
</script>

<Toaster richColors position="top-center" />

<div class="container mx-auto p-6">
	<div class="space-y-6">
		<!-- Header -->
		<div>
			<h1 class="text-3xl font-bold tracking-tight">Settings</h1>
			<p class="text-muted-foreground">Manage your settings and preferences</p>
		</div>

		<Separator />

		<!-- Main Content with Sidebar Navigation -->
		<div class="flex flex-col md:flex-row md:gap-8">
			<!-- Sidebar Navigation: full-width on mobile, fixed on desktop -->
			<div class="{mobileShowNav ? 'block' : 'hidden'} md:block">
				<SettingsNav
					items={navItems}
					activeSection={activeSection}
					onSelect={handleSectionChange}
				/>
			</div>

			<!-- Content Area -->
			<div class="{mobileShowNav ? 'hidden' : 'block'} md:block flex-1">
				<!-- Back button: mobile only -->
				<button
					class="md:hidden flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-4"
					onclick={() => mobileShowNav = true}
				>
					← Back
				</button>
				{#if activeSection === 'general'}
					<div class="space-y-6">
						<div class="space-y-2">
							<h2 class="text-2xl font-semibold">General</h2>
							<p class="text-sm text-muted-foreground">
								Customize how the app looks. Your choices are saved on this device.
							</p>
						</div>

						<!-- Color palette -->
						<div class="space-y-3">
							<div>
								<h3 class="text-lg font-medium">Color palette</h3>
								<p class="text-sm text-muted-foreground mt-1">
									Choose an accent color scheme. Background and text stay readable.
								</p>
							</div>
							<div class="grid grid-cols-2 sm:grid-cols-4 gap-3 max-w-xl">
								{#each PALETTES as p (p.key)}
									{@const selected = $palette === p.key}
									<button
										type="button"
										onclick={() => setPalette(p.key)}
										class="group rounded-lg border-2 p-3 text-left transition hover:bg-accent {selected ? 'border-ring' : 'border-border'}"
										aria-pressed={selected}
									>
										<div class="flex gap-1 mb-2">
											{#each p.swatches as hex}
												<span class="h-5 w-5 rounded" style="background-color: {hex}"></span>
											{/each}
										</div>
										<div class="text-sm font-medium">{p.label}</div>
									</button>
								{/each}
							</div>
						</div>

						<Separator />

						<!-- Appearance / mode -->
						<div class="space-y-3">
							<div>
								<h3 class="text-lg font-medium">Appearance</h3>
								<p class="text-sm text-muted-foreground mt-1">
									Switch between light and dark modes, or follow your system.
								</p>
							</div>
							<div class="flex gap-2">
								<Button
									variant={userPrefersMode.current === 'light' ? 'default' : 'outline'}
									size="sm"
									onclick={() => setMode('light')}
								>Light</Button>
								<Button
									variant={userPrefersMode.current === 'dark' ? 'default' : 'outline'}
									size="sm"
									onclick={() => setMode('dark')}
								>Dark</Button>
								<Button
									variant={userPrefersMode.current === 'system' ? 'default' : 'outline'}
									size="sm"
									onclick={() => resetMode()}
								>System</Button>
							</div>
						</div>
					</div>
				{:else if activeSection === 'account'}
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
								<Field.Label for="current_password">Current Password</Field.Label>
								<Input
									id="current_password"
									type="password"
									placeholder="Enter your password"
									bind:value={currentPassword}
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

						<!-- Two-Factor Authentication Section -->
						<div class="space-y-4">
							<div>
								<h3 class="text-lg font-medium">Two-Factor Authentication</h3>
								<p class="text-sm text-muted-foreground mt-1">
									Add an extra layer of security to your account by requiring a verification code from your authenticator app when signing in.
								</p>
							</div>

							{#if totpStatus === null}
								<div class="text-sm text-muted-foreground">Loading...</div>
							{:else if totpStatus.enabled}
								<div class="rounded-lg border p-4 space-y-4 max-w-xl">
									<div class="flex items-center justify-between">
										<div class="flex items-center gap-2">
											<span class="inline-flex items-center rounded-full bg-green-100 px-2.5 py-0.5 text-xs font-medium text-green-800 dark:bg-green-900 dark:text-green-200">
												Enabled
											</span>
											<span class="text-sm text-muted-foreground">
												{totpStatus.recovery_codes_remaining} recovery {totpStatus.recovery_codes_remaining === 1 ? 'code' : 'codes'} remaining
											</span>
										</div>
									</div>

									{#if totpStatus.recovery_codes_remaining <= 2}
										<div class="p-3 text-sm text-amber-700 bg-amber-50 border border-amber-200 rounded-md dark:bg-amber-950 dark:border-amber-800 dark:text-amber-400">
											Warning: You have very few recovery codes left. Consider regenerating them.
										</div>
									{/if}

									<div class="flex gap-2">
										<Button
											variant="outline"
											size="sm"
											onclick={() => { regenDialogOpen = true; regenCodes = null; regenPassword = ''; }}
											disabled={totpLoading}
										>
											Regenerate Recovery Codes
										</Button>
										<Button
											variant="destructive"
											size="sm"
											onclick={() => { disableDialogOpen = true; disablePassword = ''; }}
											disabled={totpLoading}
										>
											Disable 2FA
										</Button>
									</div>
								</div>
							{:else}
								<div class="rounded-lg border p-4 space-y-4 max-w-xl">
									<div class="flex items-center gap-2">
										<span class="inline-flex items-center rounded-full bg-gray-100 px-2.5 py-0.5 text-xs font-medium text-gray-800 dark:bg-gray-800 dark:text-gray-200">
											Disabled
										</span>
									</div>
									<p class="text-sm text-muted-foreground">
										Two-factor authentication is not enabled. Enable it to add an extra layer of security to your account.
									</p>
									<Button onclick={handleSetupTOTP} disabled={totpLoading}>
										{totpLoading ? 'Setting up...' : 'Enable Two-Factor Authentication'}
									</Button>
								</div>
							{/if}
						</div>
					</div>
				{/if}
			</div>
		</div>
	</div>
</div>

<!-- 2FA Setup Dialog -->
<Dialog.Root bind:open={setupDialogOpen}>
	<Dialog.Content class="max-w-md">
		{#if setupStep === 'qr' && setupData}
			<Dialog.Header>
				<Dialog.Title>Set Up Two-Factor Authentication</Dialog.Title>
				<Dialog.Description>
					Scan the QR code below with your authenticator app (e.g., Google Authenticator, Authy, 1Password).
				</Dialog.Description>
			</Dialog.Header>
			<div class="space-y-4 py-4">
				<div class="flex justify-center">
					<img
						src="data:image/png;base64,{setupData.qr_code}"
						alt="2FA QR Code"
						class="w-48 h-48"
					/>
				</div>
				<div class="space-y-2">
					<p class="text-xs text-muted-foreground text-center">
						Can't scan? Enter this key manually:
					</p>
					<code class="block text-center text-sm font-mono bg-muted p-2 rounded select-all break-all">
						{setupData.secret}
					</code>
				</div>
			</div>
			<Dialog.Footer>
				<Button variant="outline" onclick={() => { setupDialogOpen = false; }}>Cancel</Button>
				<Button onclick={() => { setupStep = 'verify'; }}>Next</Button>
			</Dialog.Footer>
		{:else if setupStep === 'verify'}
			<Dialog.Header>
				<Dialog.Title>Verify Setup</Dialog.Title>
				<Dialog.Description>
					Enter the 6-digit code from your authenticator app to verify the setup.
				</Dialog.Description>
			</Dialog.Header>
			<div class="py-4">
				<Input
					type="text"
					inputmode="numeric"
					maxlength={6}
					placeholder="000000"
					bind:value={setupVerifyCode}
					disabled={totpLoading}
					class="font-mono text-center text-2xl tracking-[0.5em]"
				/>
			</div>
			<Dialog.Footer>
				<Button variant="outline" onclick={() => { setupStep = 'qr'; }} disabled={totpLoading}>Back</Button>
				<Button onclick={handleVerifySetup} disabled={totpLoading}>
					{totpLoading ? 'Verifying...' : 'Verify & Enable'}
				</Button>
			</Dialog.Footer>
		{:else if setupStep === 'recovery' && setupData}
			<Dialog.Header>
				<Dialog.Title>Save Your Recovery Codes</Dialog.Title>
				<Dialog.Description>
					These codes can be used to access your account if you lose your authenticator device.
					Each code can only be used once. Store them somewhere safe.
				</Dialog.Description>
			</Dialog.Header>
			<div class="py-4 space-y-4">
				<div class="grid grid-cols-2 gap-2 p-4 bg-muted rounded-lg">
					{#each setupData.recovery_codes as code}
						<code class="text-sm font-mono text-center py-1">{code}</code>
					{/each}
				</div>
				<Button
					variant="outline"
					class="w-full"
					onclick={() => downloadRecoveryCodes(setupData!.recovery_codes)}
				>
					Download as Text File
				</Button>
				<p class="text-xs text-destructive text-center font-medium">
					These codes will not be shown again.
				</p>
			</div>
			<Dialog.Footer>
				<Button onclick={handleSetupComplete}>I Have Saved These Codes</Button>
			</Dialog.Footer>
		{/if}
	</Dialog.Content>
</Dialog.Root>

<!-- Disable 2FA Dialog -->
<Dialog.Root bind:open={disableDialogOpen}>
	<Dialog.Content class="max-w-sm">
		<Dialog.Header>
			<Dialog.Title>Disable Two-Factor Authentication</Dialog.Title>
			<Dialog.Description>
				Enter your password to confirm disabling 2FA. This will make your account less secure.
			</Dialog.Description>
		</Dialog.Header>
		<div class="py-4">
			<Input
				type="password"
				placeholder="Current password"
				bind:value={disablePassword}
				disabled={totpLoading}
			/>
		</div>
		<Dialog.Footer>
			<Button variant="outline" onclick={() => { disableDialogOpen = false; }} disabled={totpLoading}>Cancel</Button>
			<Button variant="destructive" onclick={handleDisableTOTP} disabled={totpLoading}>
				{totpLoading ? 'Disabling...' : 'Disable 2FA'}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Regenerate Recovery Codes Dialog -->
<Dialog.Root bind:open={regenDialogOpen}>
	<Dialog.Content class="max-w-md">
		{#if regenCodes}
			<Dialog.Header>
				<Dialog.Title>New Recovery Codes</Dialog.Title>
				<Dialog.Description>
					Your old recovery codes have been invalidated. Save these new codes in a safe place.
				</Dialog.Description>
			</Dialog.Header>
			<div class="py-4 space-y-4">
				<div class="grid grid-cols-2 gap-2 p-4 bg-muted rounded-lg">
					{#each regenCodes as code}
						<code class="text-sm font-mono text-center py-1">{code}</code>
					{/each}
				</div>
				<Button
					variant="outline"
					class="w-full"
					onclick={() => downloadRecoveryCodes(regenCodes!)}
				>
					Download as Text File
				</Button>
				<p class="text-xs text-destructive text-center font-medium">
					These codes will not be shown again.
				</p>
			</div>
			<Dialog.Footer>
				<Button onclick={handleRegenComplete}>I Have Saved These Codes</Button>
			</Dialog.Footer>
		{:else}
			<Dialog.Header>
				<Dialog.Title>Regenerate Recovery Codes</Dialog.Title>
				<Dialog.Description>
					This will invalidate all existing recovery codes. Enter your password to confirm.
				</Dialog.Description>
			</Dialog.Header>
			<div class="py-4">
				<Input
					type="password"
					placeholder="Current password"
					bind:value={regenPassword}
					disabled={totpLoading}
				/>
			</div>
			<Dialog.Footer>
				<Button variant="outline" onclick={() => { regenDialogOpen = false; }} disabled={totpLoading}>Cancel</Button>
				<Button onclick={handleRegenerateRecoveryCodes} disabled={totpLoading}>
					{totpLoading ? 'Regenerating...' : 'Regenerate Codes'}
				</Button>
			</Dialog.Footer>
		{/if}
	</Dialog.Content>
</Dialog.Root>
