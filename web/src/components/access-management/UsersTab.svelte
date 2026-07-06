<script lang="ts">
	import DataTable from '../data-table.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Select from '$lib/components/ui/select';
	import * as Dialog from '$lib/components/ui/dialog';
	import { toast } from 'svelte-sonner';
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

	interface Props {
		active: boolean;
	}
	let { active }: Props = $props();

	const UNASSIGNED = '__unassigned__';

	let users = $state<UserAccess[]>([]);
	let groups = $state<Group[]>([]);
	let features = $state<Feature[]>([]);
	let loading = $state(false);
	let error = $state('');
	let reassigningUserId = $state<string | null>(null);

	// Admin-only features (access-management) are gated by admin-group
	// membership, not a feature grant/override check — an override on them
	// would never be consulted, so they're excluded here (mirrors GroupsTab).
	let overridableFeatures = $derived(features.filter((f) => !f.admin_only));

	let overridesDialogOpen = $state(false);
	let overridesUser = $state<UserAccess | null>(null);
	let overrideState = $state<Record<string, 'inherit' | 'allow' | 'deny'>>({});
	let savingOverrides = $state(false);

	// Re-fetch every time this tab becomes active — bits-ui's Tabs.Content
	// never unmounts inactive panels, so a plain onMount would only ever run
	// once for this component's whole lifetime (see AccessManagement.svelte).
	$effect(() => {
		if (active) load();
	});

	async function load() {
		loading = true;
		error = '';
		try {
			[users, groups, features] = await Promise.all([listUsers(), listGroups(), listFeatures()]);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load users';
		} finally {
			loading = false;
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

	const columns: ColumnDef<UserAccess>[] = [
		// id explicitly set to "name" (rather than defaulting to "username") so
		// DataTable's special-cased nameColumn snippet renders the admin badge —
		// it only hooks custom Svelte content for columns literally named
		// "name" or "actions" (see components/data-table.svelte).
		{ accessorKey: 'username', id: 'name', header: 'Username', cell: (info) => info.getValue() },
		{ accessorKey: 'email', header: 'Email', cell: (info) => info.getValue() },
		{ id: 'actions', header: 'Group / Overrides', enableHiding: false }
	];
</script>

<div class="space-y-4">
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
			</div>
		{/snippet}
	</DataTable>
</div>

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
