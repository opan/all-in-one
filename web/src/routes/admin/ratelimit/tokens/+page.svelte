<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button/index';
	import { Input } from '$lib/components/ui/input/index';
	import { Label } from '$lib/components/ui/label/index';
	import * as Dialog from '$lib/components/ui/dialog/index';
	import * as AlertDialog from '$lib/components/ui/alert-dialog/index';
	import * as Table from '$lib/components/ui/table/index';
	import { toast, Toaster } from 'svelte-sonner';
	import { Loader2 } from '@lucide/svelte/icons';
	import {
		listTokens,
		createToken,
		revokeToken,
		type AppToken,
		type CreateTokenResponse
	} from '$lib/ratelimit-api';

	let tokens = $state<AppToken[]>([]);
	let loading = $state(true);
	let error = $state('');

	// Create dialog
	let createDialogOpen = $state(false);
	let formApp = $state('');
	let formName = $state('');
	let formScope = $state('');
	let creating = $state(false);

	// Plaintext-shown-once dialog
	let secretDialogOpen = $state(false);
	let createdToken = $state<CreateTokenResponse | null>(null);

	// Revoke confirmation
	let revokeDialogOpen = $state(false);
	let revokingTarget = $state<AppToken | null>(null);
	let revoking = $state(false);

	onMount(load);

	async function load() {
		loading = true;
		error = '';
		try {
			tokens = await listTokens();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load app tokens';
		} finally {
			loading = false;
		}
	}

	function openCreateDialog() {
		formApp = '';
		formName = '';
		formScope = '';
		createDialogOpen = true;
	}

	async function handleCreateSubmit(e: SubmitEvent) {
		e.preventDefault();
		if (!formApp.trim() || !formName.trim()) return;
		creating = true;
		try {
			const created = await createToken({
				app: formApp.trim(),
				name: formName.trim(),
				scope_prefix: formScope.trim() || undefined
			});
			createDialogOpen = false;
			createdToken = created;
			secretDialogOpen = true;
			await load();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to create token');
		} finally {
			creating = false;
		}
	}

	async function copyToken() {
		if (!createdToken) return;
		try {
			await navigator.clipboard.writeText(createdToken.token);
			toast.success('Token copied to clipboard');
		} catch {
			toast.error('Could not copy — select and copy manually');
		}
	}

	function openRevokeDialog(token: AppToken) {
		revokingTarget = token;
		revokeDialogOpen = true;
	}

	async function handleRevoke() {
		if (!revokingTarget) return;
		revoking = true;
		try {
			await revokeToken(revokingTarget.id);
			toast.success('Token revoked');
			revokeDialogOpen = false;
			revokingTarget = null;
			await load();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to revoke token');
		} finally {
			revoking = false;
		}
	}

	function fmtDate(s?: string): string {
		if (!s) return '—';
		return new Date(s).toLocaleString();
	}
</script>

<Toaster richColors position="top-center" />

<div class="container mx-auto space-y-6 p-6">
	<div class="flex items-start justify-between">
		<div class="space-y-2">
			<h1 class="text-3xl font-bold tracking-tight">App Tokens</h1>
			<p class="text-muted-foreground">
				Service credentials other apps use to call the rate-limit check API. The plaintext
				token is shown once, at creation.
			</p>
		</div>
		<Button onclick={openCreateDialog}>Create token</Button>
	</div>

	{#if error}
		<div class="rounded-md bg-destructive/15 p-3 text-sm text-destructive">{error}</div>
	{/if}

	<div class="rounded-md border">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head>App</Table.Head>
					<Table.Head>Name</Table.Head>
					<Table.Head>Prefix</Table.Head>
					<Table.Head>Scope</Table.Head>
					<Table.Head>Created</Table.Head>
					<Table.Head>Last used</Table.Head>
					<Table.Head class="w-24">Status</Table.Head>
					<Table.Head class="w-24 text-right">Actions</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#if !loading && tokens.length === 0}
					<Table.Row>
						<Table.Cell colspan={8} class="h-24 text-center text-muted-foreground">
							No app tokens yet.
						</Table.Cell>
					</Table.Row>
				{:else}
					{#each tokens as token (token.id)}
						<Table.Row class={token.revoked_at ? 'opacity-50' : ''}>
							<Table.Cell class="font-medium">{token.app}</Table.Cell>
							<Table.Cell>{token.name}</Table.Cell>
							<Table.Cell class="font-mono text-xs">{token.token_prefix}…</Table.Cell>
							<Table.Cell class="font-mono text-xs">{token.scope_prefix}</Table.Cell>
							<Table.Cell class="text-sm text-muted-foreground">{fmtDate(token.created_at)}</Table.Cell>
							<Table.Cell class="text-sm text-muted-foreground">{fmtDate(token.last_used_at)}</Table.Cell>
							<Table.Cell>
								{#if token.revoked_at}
									<span class="inline-flex items-center rounded-full bg-destructive/15 px-2 py-0.5 text-xs font-medium text-destructive">revoked</span>
								{:else}
									<span class="inline-flex items-center rounded-full bg-emerald-500/15 px-2 py-0.5 text-xs font-medium text-emerald-600">active</span>
								{/if}
							</Table.Cell>
							<Table.Cell class="text-right">
								{#if !token.revoked_at}
									<Button variant="outline" size="sm" onclick={() => openRevokeDialog(token)}>
										Revoke
									</Button>
								{/if}
							</Table.Cell>
						</Table.Row>
					{/each}
				{/if}
			</Table.Body>
		</Table.Root>
	</div>
</div>

<!-- Create token -->
<Dialog.Root bind:open={createDialogOpen}>
	<Dialog.Content class="max-w-md">
		<Dialog.Header>
			<Dialog.Title>Create app token</Dialog.Title>
			<Dialog.Description>
				Mint a token for another app. Its scope prefix limits which target keys it may check.
			</Dialog.Description>
		</Dialog.Header>
		<form onsubmit={handleCreateSubmit} class="space-y-4">
			<div class="space-y-2">
				<Label for="token-app">App</Label>
				<Input id="token-app" bind:value={formApp} placeholder="cashflow" required />
			</div>
			<div class="space-y-2">
				<Label for="token-name">Name</Label>
				<Input id="token-name" bind:value={formName} placeholder="cashflow prod" required />
			</div>
			<div class="space-y-2">
				<Label for="token-scope">Scope prefix (optional)</Label>
				<Input id="token-scope" bind:value={formScope} placeholder="defaults to <app>." />
			</div>
			<Dialog.Footer>
				<Button type="button" variant="outline" onclick={() => (createDialogOpen = false)}>
					Cancel
				</Button>
				<Button type="submit" disabled={creating}>
					{#if creating}<Loader2 class="mr-2 size-4 animate-spin" />{/if}
					Create
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>

<!-- Plaintext shown once -->
<Dialog.Root bind:open={secretDialogOpen}>
	<Dialog.Content class="max-w-lg">
		<Dialog.Header>
			<Dialog.Title>Token created</Dialog.Title>
			<Dialog.Description>
				Copy this token now — it will <strong>not</strong> be shown again.
			</Dialog.Description>
		</Dialog.Header>
		<div class="space-y-3">
			<div class="rounded-md border bg-muted p-3 font-mono text-sm break-all">
				{createdToken?.token}
			</div>
			<Button variant="outline" onclick={copyToken}>Copy to clipboard</Button>
		</div>
		<Dialog.Footer>
			<Button onclick={() => (secretDialogOpen = false)}>Done</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Revoke confirmation -->
<AlertDialog.Root bind:open={revokeDialogOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Revoke this token?</AlertDialog.Title>
			<AlertDialog.Description>
				{revokingTarget?.name} ({revokingTarget?.app}) will stop working immediately. This cannot
				be undone; mint a new token to replace it.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={revoking}>Cancel</AlertDialog.Cancel>
			<Button variant="destructive" disabled={revoking} onclick={handleRevoke}>
				{#if revoking}<Loader2 class="mr-2 size-4 animate-spin" />{/if}
				Revoke
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
