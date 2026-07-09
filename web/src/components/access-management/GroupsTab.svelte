<script lang="ts">
	import DataTable from '../data-table.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { toast } from 'svelte-sonner';
	import type { ColumnDef } from '@tanstack/table-core';
	import {
		listGroups,
		listFeatures,
		createGroup,
		updateGroup,
		deleteGroup,
		setGroupFeatures,
		type Group,
		type Feature
	} from '$lib/rbac-api';

	interface Props {
		active: boolean;
	}
	let { active }: Props = $props();

	let groups = $state<Group[]>([]);
	let features = $state<Feature[]>([]);
	let loading = $state(false);
	let error = $state('');

	// Admin-only features (e.g. access-management) aren't grantable through
	// group_features — access-management is gated by admin-group membership
	// (RequireAdmin), not a feature grant, so a checkbox for it would be inert.
	let grantableFeatures = $derived(features.filter((f) => !f.admin_only));

	let dialogOpen = $state(false);
	let editingGroup = $state<Group | null>(null);
	let formName = $state('');
	let formDescription = $state('');
	let formFeatureKeys = $state<string[]>([]);
	let saving = $state(false);

	let deleteDialogOpen = $state(false);
	let groupToDelete = $state<Group | null>(null);
	let deleting = $state(false);

	// Re-fetch every time this tab becomes active — bits-ui's Tabs.Content
	// never unmounts inactive panels, so a plain onMount would only ever run
	// once for this component's whole lifetime (see routes/admin/access/+page.svelte).
	$effect(() => {
		if (active) load();
	});

	async function load() {
		loading = true;
		error = '';
		try {
			[groups, features] = await Promise.all([listGroups(), listFeatures()]);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load groups';
		} finally {
			loading = false;
		}
	}

	function openCreateDialog() {
		editingGroup = null;
		formName = '';
		formDescription = '';
		formFeatureKeys = [];
		dialogOpen = true;
	}

	function openEditDialog(group: Group) {
		editingGroup = group;
		formName = group.name;
		formDescription = group.description || '';
		formFeatureKeys = [...(group.feature_keys || [])];
		dialogOpen = true;
	}

	function toggleFeature(key: string, checked: boolean) {
		formFeatureKeys = checked
			? [...formFeatureKeys, key]
			: formFeatureKeys.filter((k) => k !== key);
	}

	async function handleSubmit(e: Event) {
		e.preventDefault();
		if (!formName.trim()) {
			toast.error('Name is required');
			return;
		}

		saving = true;
		try {
			if (editingGroup) {
				await updateGroup(editingGroup.id, { name: formName.trim(), description: formDescription.trim() });
				await setGroupFeatures(editingGroup.id, formFeatureKeys);
				toast.success('Group updated');
			} else {
				await createGroup({
					name: formName.trim(),
					description: formDescription.trim(),
					feature_keys: formFeatureKeys
				});
				toast.success('Group created');
			}
			dialogOpen = false;
			await load();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to save group');
		} finally {
			saving = false;
		}
	}

	function openDeleteDialog(group: Group) {
		groupToDelete = group;
		deleteDialogOpen = true;
	}

	async function confirmDelete() {
		if (!groupToDelete) return;
		deleting = true;
		try {
			await deleteGroup(groupToDelete.id);
			toast.success('Group deleted');
			deleteDialogOpen = false;
			groupToDelete = null;
			await load();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to delete group');
		} finally {
			deleting = false;
		}
	}

	const columns: ColumnDef<Group>[] = [
		{ accessorKey: 'name', header: 'Name', cell: (info) => info.getValue() },
		{
			accessorKey: 'description',
			header: 'Description',
			cell: (info) => info.getValue() || '—'
		},
		{
			id: 'features',
			header: 'Granted features',
			cell: (info) => (info.row.original.feature_keys || []).join(', ') || '—'
		},
		{ id: 'actions', header: 'Actions', enableHiding: false }
	];
</script>

<div class="space-y-4">
	{#if error}
		<div class="rounded-md bg-destructive/15 p-3 text-sm text-destructive">{error}</div>
	{/if}

	<DataTable
		data={groups}
		{columns}
		filterPlaceholder="Filter groups..."
		showFilter={true}
		showColumnVisibility={false}
		showPagination={false}
		onReload={load}
		actions={[{ label: 'New Group', onclick: openCreateDialog }]}
	>
		{#snippet nameColumn({ row })}
			<div class="flex items-center gap-2">
				<span class="font-medium">{row.original.name}</span>
				{#if row.original.is_builtin}
					<span class="inline-flex items-center rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
						Built-in
					</span>
				{/if}
			</div>
		{/snippet}
		{#snippet actionsColumn({ row })}
			<div class="flex justify-end gap-2">
				<Button variant="outline" size="sm" onclick={() => openEditDialog(row.original)}>Edit</Button>
				<Button
					variant="destructive"
					size="sm"
					disabled={row.original.is_builtin}
					title={row.original.is_builtin ? 'Built-in groups cannot be deleted' : undefined}
					onclick={() => openDeleteDialog(row.original)}
				>
					Delete
				</Button>
			</div>
		{/snippet}
	</DataTable>
</div>

<Dialog.Root bind:open={dialogOpen}>
	<Dialog.Content class="max-w-md">
		<Dialog.Header>
			<Dialog.Title>{editingGroup ? 'Edit Group' : 'New Group'}</Dialog.Title>
			<Dialog.Description>
				{editingGroup
					? 'Update this group\'s name, description, and granted features.'
					: 'Create a permission-preset group and choose which features it grants.'}
			</Dialog.Description>
		</Dialog.Header>
		<form onsubmit={handleSubmit} class="space-y-4">
			<div class="space-y-2">
				<Label for="group-name">Name</Label>
				<Input
					id="group-name"
					bind:value={formName}
					disabled={editingGroup?.is_builtin}
					required
				/>
				{#if editingGroup?.is_builtin}
					<p class="text-xs text-muted-foreground">Built-in group names cannot be changed.</p>
				{/if}
			</div>
			<div class="space-y-2">
				<Label for="group-description">Description</Label>
				<Input id="group-description" bind:value={formDescription} />
			</div>
			<div class="space-y-2">
				<Label>Granted features</Label>
				{#if editingGroup?.name === 'admin'}
					<p class="text-xs text-muted-foreground">
						Admin bypasses all feature gates — its grants have no effect and cannot be edited here.
					</p>
				{:else}
					<div class="space-y-2 rounded-md border p-3">
						{#each grantableFeatures as feature (feature.id)}
							<div class="flex items-center space-x-2">
								<Checkbox
									id="feature-{feature.key}"
									checked={formFeatureKeys.includes(feature.key)}
									onCheckedChange={(checked) => toggleFeature(feature.key, checked === true)}
								/>
								<Label for="feature-{feature.key}" class="font-normal">{feature.name}</Label>
							</div>
						{/each}
					</div>
				{/if}
			</div>
			<Dialog.Footer>
				<Button type="button" variant="outline" onclick={() => (dialogOpen = false)} disabled={saving}>
					Cancel
				</Button>
				<Button type="submit" disabled={saving}>
					{saving ? 'Saving...' : editingGroup ? 'Save changes' : 'Create group'}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>

<AlertDialog.Root bind:open={deleteDialogOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete Group</AlertDialog.Title>
			<AlertDialog.Description>
				{#if groupToDelete}
					Are you sure you want to delete <span class="font-semibold">"{groupToDelete.name}"</span>?
					Members of this group will revert to the regular-user default.
				{/if}
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel
				onclick={() => { deleteDialogOpen = false; groupToDelete = null; }}
				disabled={deleting}
			>
				Cancel
			</AlertDialog.Cancel>
			<AlertDialog.Action
				onclick={confirmDelete}
				disabled={deleting}
				class="bg-destructive text-destructive-foreground hover:bg-destructive/90"
			>
				{deleting ? 'Deleting...' : 'Delete'}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
