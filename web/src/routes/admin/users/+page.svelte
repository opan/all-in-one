<script lang="ts">
	import { onMount } from 'svelte';
	import DataTable from '../../../components/data-table.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { toast, Toaster } from 'svelte-sonner';
	import type { ColumnDef } from '@tanstack/table-core';
	import {
		listUsers,
		listGroups,
		listFeatures,
		assignUserGroup,
		getUserOverrides,
		setUserOverrides,
		type UserAccess,
		type Group,
		type Feature
	} from '$lib/rbac-api';
	import { updateUserEmail, blockUser, unblockUser } from '$lib/admin-api';

	const UNASSIGNED = '__unassigned__';

	let users = $state<UserAccess[]>([]);
	let groups = $state<Group[]>([]);
	let features = $state<Feature[]>([]);
	let error = $state('');
	let reassigningUserId = $state<string | null>(null);

	// Admin-only features (access-management) are gated by admin-group membership,
	// not a feature grant/override — an override on them is never consulted, so
	// they're excluded here (mirrors GroupsTab / the old UsersTab).
	let overridableFeatures = $derived(features.filter((f) => !f.admin_only));

	// Overrides dialog
	let overridesDialogOpen = $state(false);
	let overridesUser = $state<UserAccess | null>(null);
	let overrideState = $state<Record<string, 'inherit' | 'allow' | 'deny'>>({});
	let savingOverrides = $state(false);

	// Edit-email dialog
	let emailDialogOpen = $state(false);
	let emailUser = $state<UserAccess | null>(null);
	let emailValue = $state('');
	let savingEmail = $state(false);

	// Block confirmation
	let blockDialogOpen = $state(false);
	let blockUserTarget = $state<UserAccess | null>(null);
	let blocking = $state(false);

	// This is a route (not a bits-ui tab), so it mounts fresh on navigation — a
	// plain onMount is correct here; no active-prop refetch hack needed.
	onMount(load);

	async function load() {
		error = '';
		try {
			[users, groups, features] = await Promise.all([listUsers(), listGroups(), listFeatures()]);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load users';
		}
	}

	async function handleGroupChange(user: UserAccess, value: string | undefined) {
		if (!value) return;
		const groupId = value === UNASSIGNED ? null : value;
		if (groupId === user.group_id) return;

		reassigningUserId = user.user_id;
		try {
			await assignUserGroup(user.user_id, groupId);
			toast.success(`${user.username}'s group updated`);
			await load();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to reassign group');
		} finally {
			reassigningUserId = null;
		}
	}

	async function openOverridesDialog(user: UserAccess) {
		overridesUser = user;
		overridesDialogOpen = true;
		overrideState = Object.fromEntries(overridableFeatures.map((f) => [f.key, 'inherit' as const]));
		try {
			const current = await getUserOverrides(user.user_id);
			for (const ov of current) {
				overrideState[ov.feature_key] = ov.allow ? 'allow' : 'deny';
			}
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to load overrides');
		}
	}

	async function saveOverrides() {
		if (!overridesUser) return;
		savingOverrides = true;
		try {
			const overrides = Object.entries(overrideState)
				.filter(([, state]) => state !== 'inherit')
				.map(([feature_key, state]) => ({ feature_key, allow: state === 'allow' }));
			await setUserOverrides(overridesUser.user_id, overrides);
			toast.success('Overrides updated');
			overridesDialogOpen = false;
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to save overrides');
		} finally {
			savingOverrides = false;
		}
	}

	function openEmailDialog(user: UserAccess) {
		emailUser = user;
		emailValue = user.email;
		emailDialogOpen = true;
	}

	async function saveEmail() {
		if (!emailUser) return;
		savingEmail = true;
		try {
			await updateUserEmail(emailUser.user_id, emailValue.trim());
			toast.success('Email updated');
			emailDialogOpen = false;
			await load();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to update email');
		} finally {
			savingEmail = false;
		}
	}

	function openBlockDialog(user: UserAccess) {
		blockUserTarget = user;
		blockDialogOpen = true;
	}

	async function confirmBlock() {
		if (!blockUserTarget) return;
		blocking = true;
		try {
			await blockUser(blockUserTarget.user_id);
			toast.success(`${blockUserTarget.username} has been blocked`);
			blockDialogOpen = false;
			blockUserTarget = null;
			await load();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to block user');
		} finally {
			blocking = false;
		}
	}

	async function handleUnblock(user: UserAccess) {
		try {
			await unblockUser(user.user_id);
			toast.success(`${user.username} has been unblocked`);
			await load();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to unblock user');
		}
	}

	const columns: ColumnDef<UserAccess>[] = [
		// id "name"/"actions" so DataTable renders the custom snippets below.
		{ accessorKey: 'username', id: 'name', header: 'Username', cell: (info) => info.getValue() },
		{ accessorKey: 'email', header: 'Email', cell: (info) => info.getValue() },
		{ id: 'actions', header: 'Group / Actions', enableHiding: false }
	];
</script>

<Toaster richColors position="top-center" />

<div class="container mx-auto p-6 space-y-6">
	<div class="space-y-2">
		<h1 class="text-3xl font-bold tracking-tight">Users</h1>
		<p class="text-muted-foreground">
			Manage users: edit email, block or unblock login, assign a group, and set per-user overrides.
		</p>
	</div>

	{#if error}
		<div class="rounded-md bg-destructive/15 p-3 text-sm text-destructive">{error}</div>
	{/if}

	<DataTable
		data={users}
		{columns}
		filterPlaceholder="Filter users..."
		showFilter={true}
		showColumnVisibility={false}
		showPagination={false}
		onReload={load}
	>
		{#snippet nameColumn({ row })}
			<div class="flex items-center gap-2">
				<span class="font-medium">{row.original.username}</span>
				{#if row.original.is_admin}
					<span class="inline-flex items-center rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-800 dark:bg-blue-900 dark:text-blue-200">
						Admin
					</span>
				{/if}
				{#if row.original.blocked}
					<span class="inline-flex items-center rounded-full bg-red-100 px-2 py-0.5 text-xs font-medium text-red-800 dark:bg-red-900 dark:text-red-200">
						Blocked
					</span>
				{/if}
			</div>
		{/snippet}
		{#snippet actionsColumn({ row })}
			<div class="flex justify-end items-center gap-2">
				<Select.Root
					type="single"
					value={row.original.group_id ?? UNASSIGNED}
					onValueChange={(v) => handleGroupChange(row.original, v)}
					disabled={reassigningUserId === row.original.user_id}
				>
					<Select.Trigger class="w-40">
						{row.original.group_name ?? 'regular-user (default)'}
					</Select.Trigger>
					<Select.Content>
						<Select.Item value={UNASSIGNED} label="regular-user (default)">
							regular-user (default)
						</Select.Item>
						{#each groups as group (group.id)}
							<Select.Item value={group.id} label={group.name}>{group.name}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
				<Button variant="outline" size="sm" onclick={() => openOverridesDialog(row.original)}>
					Overrides
				</Button>
				<DropdownMenu.Root>
					<DropdownMenu.Trigger>
						{#snippet child({ props })}
							<Button {...props} variant="ghost" size="sm" aria-label="More actions">⋯</Button>
						{/snippet}
					</DropdownMenu.Trigger>
					<DropdownMenu.Content align="end">
						<DropdownMenu.Item onclick={() => openEmailDialog(row.original)}>
							Edit email
						</DropdownMenu.Item>
						{#if !row.original.is_admin}
							<DropdownMenu.Separator />
							{#if row.original.blocked}
								<DropdownMenu.Item onclick={() => handleUnblock(row.original)}>
									Unblock login
								</DropdownMenu.Item>
							{:else}
								<DropdownMenu.Item
									class="text-destructive data-highlighted:text-destructive"
									onclick={() => openBlockDialog(row.original)}
								>
									Block login
								</DropdownMenu.Item>
							{/if}
						{/if}
					</DropdownMenu.Content>
				</DropdownMenu.Root>
			</div>
		{/snippet}
	</DataTable>
</div>

<!-- Edit email dialog -->
<Dialog.Root bind:open={emailDialogOpen}>
	<Dialog.Content class="max-w-md">
		<Dialog.Header>
			<Dialog.Title>Edit email — {emailUser?.username}</Dialog.Title>
			<Dialog.Description>Change this user's email address.</Dialog.Description>
		</Dialog.Header>
		<div class="space-y-2">
			<Label for="edit-email">Email</Label>
			<Input id="edit-email" type="email" bind:value={emailValue} />
		</div>
		<Dialog.Footer>
			<Button variant="outline" onclick={() => (emailDialogOpen = false)} disabled={savingEmail}>
				Cancel
			</Button>
			<Button onclick={saveEmail} disabled={savingEmail}>
				{savingEmail ? 'Saving...' : 'Save'}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Overrides dialog -->
<Dialog.Root bind:open={overridesDialogOpen}>
	<Dialog.Content class="max-w-md">
		<Dialog.Header>
			<Dialog.Title>Feature Overrides — {overridesUser?.username}</Dialog.Title>
			<Dialog.Description>
				Per-feature grant/revoke, takes precedence over the user's group. "Inherit" removes the
				override and falls back to the group's grant.
			</Dialog.Description>
		</Dialog.Header>
		<div class="space-y-3">
			{#each overridableFeatures as feature (feature.id)}
				<div class="flex items-center justify-between gap-4">
					<span class="text-sm font-medium">{feature.name}</span>
					<Select.Root
						type="single"
						value={overrideState[feature.key]}
						onValueChange={(v) => {
							if (v) overrideState[feature.key] = v as 'inherit' | 'allow' | 'deny';
						}}
					>
						<Select.Trigger class="w-32">
							{overrideState[feature.key] === 'allow'
								? 'Allow'
								: overrideState[feature.key] === 'deny'
									? 'Deny'
									: 'Inherit'}
						</Select.Trigger>
						<Select.Content>
							<Select.Item value="inherit" label="Inherit">Inherit</Select.Item>
							<Select.Item value="allow" label="Allow">Allow</Select.Item>
							<Select.Item value="deny" label="Deny">Deny</Select.Item>
						</Select.Content>
					</Select.Root>
				</div>
			{/each}
		</div>
		<Dialog.Footer>
			<Button variant="outline" onclick={() => (overridesDialogOpen = false)} disabled={savingOverrides}>
				Cancel
			</Button>
			<Button onclick={saveOverrides} disabled={savingOverrides}>
				{savingOverrides ? 'Saving...' : 'Save'}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Block confirmation -->
<AlertDialog.Root bind:open={blockDialogOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Block login</AlertDialog.Title>
			<AlertDialog.Description>
				{#if blockUserTarget}
					Block <span class="font-semibold">{blockUserTarget.username}</span>? They will be signed out
					of all sessions immediately and cannot log back in until unblocked.
				{/if}
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel onclick={() => { blockDialogOpen = false; blockUserTarget = null; }} disabled={blocking}>
				Cancel
			</AlertDialog.Cancel>
			<AlertDialog.Action
				onclick={confirmBlock}
				disabled={blocking}
				class="bg-destructive text-destructive-foreground hover:bg-destructive/90"
			>
				{blocking ? 'Blocking...' : 'Block'}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
