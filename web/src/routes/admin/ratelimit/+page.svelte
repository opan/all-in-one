<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button/index';
	import { Input } from '$lib/components/ui/input/index';
	import { Label } from '$lib/components/ui/label/index';
	import { Switch } from '$lib/components/ui/switch/index';
	import * as Select from '$lib/components/ui/select/index';
	import * as Dialog from '$lib/components/ui/dialog/index';
	import * as AlertDialog from '$lib/components/ui/alert-dialog/index';
	import * as Table from '$lib/components/ui/table/index';
	import { toast, Toaster } from 'svelte-sonner';
	import { Loader2 } from '@lucide/svelte/icons';
	import {
		listTargets,
		updateTarget,
		resetCounters,
		resetDefaults,
		createExternalTarget,
		deleteExternalTarget,
		type RateLimitTarget,
		type WindowUnit,
		type Scope,
		type Kind
	} from '$lib/ratelimit-api';

	const WINDOW_UNITS: WindowUnit[] = ['second', 'minute', 'hour', 'day'];
	const SCOPES: Scope[] = ['ip', 'user', 'global'];
	const KINDS: Kind[] = ['throttle', 'daily_quota'];

	let targets = $state<RateLimitTarget[]>([]);
	let loading = $state(true);
	let error = $state('');

	// Targets grouped by app for section headers. Internal (all-in-one) first,
	// then external apps alphabetically.
	let groupedTargets = $derived.by(() => {
		const byApp = new Map<string, RateLimitTarget[]>();
		for (const t of targets) {
			const app = t.app || 'all-in-one';
			(byApp.get(app) ?? byApp.set(app, []).get(app)!).push(t);
		}
		return [...byApp.entries()].sort(([a], [b]) => {
			if (a === 'all-in-one') return -1;
			if (b === 'all-in-one') return 1;
			return a.localeCompare(b);
		});
	});

	// Per-row toggle loading
	let toggling = $state<Record<string, boolean>>({});

	// Create external target dialog
	let createDialogOpen = $state(false);
	let cForm = $state({
		key: '',
		app: '',
		name: '',
		description: '',
		scope: 'user' as Scope,
		kind: 'daily_quota' as Kind,
		limit_count: 1000,
		window_value: 1,
		window_unit: 'day' as WindowUnit
	});
	let creating = $state(false);

	// Delete external target confirmation
	let deleteDialogOpen = $state(false);
	let deleteTarget = $state<RateLimitTarget | null>(null);
	let deleting = $state(false);

	// Edit dialog
	let editDialogOpen = $state(false);
	let editingTarget = $state<RateLimitTarget | null>(null);
	let formLimit = $state(1);
	let formWindowValue = $state(1);
	let formWindowUnit = $state<WindowUnit>('minute');
	let saving = $state(false);

	// Reset confirmation (shared between "reset counters" and "reset to defaults")
	let resetDialogOpen = $state(false);
	let resetTarget = $state<RateLimitTarget | null>(null);
	let resetAction = $state<'reset' | 'reset-defaults'>('reset');
	let resetting = $state(false);

	onMount(load);

	async function load() {
		loading = true;
		error = '';
		try {
			targets = await listTargets();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load rate limit targets';
		} finally {
			loading = false;
		}
	}

	async function handleToggleEnabled(target: RateLimitTarget) {
		toggling = { ...toggling, [target.key]: true };
		try {
			const updated = await updateTarget(target.key, { enabled: !target.enabled });
			targets = targets.map((t) => (t.key === target.key ? updated : t));
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to update rate limit target');
		} finally {
			toggling = { ...toggling, [target.key]: false };
		}
	}

	function openEditDialog(target: RateLimitTarget) {
		editingTarget = target;
		formLimit = target.limit_count;
		formWindowValue = target.window_value;
		formWindowUnit = target.window_unit;
		editDialogOpen = true;
	}

	async function handleEditSubmit(e: Event) {
		e.preventDefault();
		if (!editingTarget) return;
		if (formLimit < 1) {
			toast.error('Limit must be at least 1');
			return;
		}
		if (formWindowValue < 1) {
			toast.error('Window value must be at least 1');
			return;
		}

		saving = true;
		try {
			const updated = await updateTarget(editingTarget.key, {
				limit_count: formLimit,
				window_value: formWindowValue,
				window_unit: formWindowUnit
			});
			targets = targets.map((t) => (t.key === updated.key ? updated : t));
			toast.success(`${editingTarget.name} updated`);
			editDialogOpen = false;
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to update rate limit target');
		} finally {
			saving = false;
		}
	}

	function openResetDialog(target: RateLimitTarget, action: 'reset' | 'reset-defaults') {
		resetTarget = target;
		resetAction = action;
		resetDialogOpen = true;
	}

	async function confirmReset() {
		if (!resetTarget) return;
		resetting = true;
		try {
			if (resetAction === 'reset') {
				await resetCounters(resetTarget.key);
				toast.success(`Counters reset for ${resetTarget.name}`);
			} else {
				const updated = await resetDefaults(resetTarget.key);
				targets = targets.map((t) => (t.key === updated.key ? updated : t));
				toast.success(`${resetTarget.name} reset to defaults`);
			}
			resetDialogOpen = false;
			resetTarget = null;
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to reset rate limit target');
		} finally {
			resetting = false;
		}
	}

	function openCreateDialog() {
		cForm = {
			key: '',
			app: '',
			name: '',
			description: '',
			scope: 'user',
			kind: 'daily_quota',
			limit_count: 1000,
			window_value: 1,
			window_unit: 'day'
		};
		createDialogOpen = true;
	}

	async function handleCreateSubmit(e: Event) {
		e.preventDefault();
		if (!cForm.key.trim()) {
			toast.error('Target key is required');
			return;
		}
		if (cForm.limit_count < 1 || cForm.window_value < 1) {
			toast.error('Limit and window value must be at least 1');
			return;
		}
		creating = true;
		try {
			const created = await createExternalTarget({
				key: cForm.key.trim(),
				app: cForm.app.trim() || undefined,
				name: cForm.name.trim() || undefined,
				description: cForm.description.trim() || undefined,
				scope: cForm.scope,
				kind: cForm.kind,
				limit_count: cForm.limit_count,
				window_value: cForm.window_value,
				window_unit: cForm.window_unit,
				enabled: true
			});
			targets = [...targets, created];
			toast.success(`External target ${created.key} created`);
			createDialogOpen = false;
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to create external target');
		} finally {
			creating = false;
		}
	}

	function openDeleteDialog(target: RateLimitTarget) {
		deleteTarget = target;
		deleteDialogOpen = true;
	}

	async function confirmDelete() {
		if (!deleteTarget) return;
		deleting = true;
		try {
			await deleteExternalTarget(deleteTarget.key);
			targets = targets.filter((t) => t.key !== deleteTarget!.key);
			toast.success(`${deleteTarget.key} deleted`);
			deleteDialogOpen = false;
			deleteTarget = null;
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to delete external target');
		} finally {
			deleting = false;
		}
	}

	function scopeBadgeClass(scope: string): string {
		switch (scope) {
			case 'ip':
				return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400';
			case 'user':
				return 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400';
			default:
				return 'bg-muted text-muted-foreground';
		}
	}

	function kindBadgeClass(kind: string): string {
		return kind === 'throttle'
			? 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400'
			: 'bg-teal-100 text-teal-700 dark:bg-teal-900/30 dark:text-teal-400';
	}

	function windowLabel(value: number, unit: string): string {
		return `${value} ${value === 1 ? unit : `${unit}s`}`;
	}
</script>

<Toaster richColors position="top-center" />

<div class="container mx-auto p-6 space-y-6">
	<div class="flex items-start justify-between">
		<div class="space-y-2">
			<h1 class="text-3xl font-bold tracking-tight">Rate Limits</h1>
			<p class="text-muted-foreground">
				{targets.length} rate-limited {targets.length === 1 ? 'target' : 'targets'}, grouped by
				app. Internal targets are code-defined; external targets are created here.
			</p>
		</div>
		<Button onclick={openCreateDialog}>Create external target</Button>
	</div>

	{#if error}
		<div class="rounded-md bg-destructive/15 p-3 text-sm text-destructive">{error}</div>
	{/if}

	<div class="rounded-md border">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head>Name</Table.Head>
					<Table.Head class="w-24">Scope</Table.Head>
					<Table.Head class="w-28">Kind</Table.Head>
					<Table.Head class="w-20 text-right">Limit</Table.Head>
					<Table.Head class="w-32">Window</Table.Head>
					<Table.Head class="w-20">Enabled</Table.Head>
					<Table.Head class="w-64 text-right">Actions</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#if !loading && targets.length === 0}
					<Table.Row>
						<Table.Cell colspan={7} class="h-24 text-center text-muted-foreground">
							{#if error}
								No rate limit targets to show.
							{:else}
								No rate limit targets configured.
							{/if}
						</Table.Cell>
					</Table.Row>
				{:else}
					{#each groupedTargets as [app, appTargets] (app)}
						<Table.Row class="bg-muted/50 hover:bg-muted/50">
							<Table.Cell colspan={7} class="py-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
								{app}
								{#if app !== 'all-in-one'}
									<span class="ml-2 rounded-full bg-primary/10 px-2 py-0.5 text-[10px] font-medium normal-case text-primary">external</span>
								{/if}
							</Table.Cell>
						</Table.Row>
						{#each appTargets as target (target.key)}
							<Table.Row class={target.enabled ? '' : 'opacity-50'}>
								<Table.Cell>
									<div class="flex flex-col">
										<span class="font-medium">{target.name || target.key}</span>
										<span class="font-mono text-xs text-muted-foreground">{target.key}</span>
									</div>
								</Table.Cell>

								<Table.Cell>
									<span
										class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium {scopeBadgeClass(
											target.scope
										)}"
									>
										{target.scope}
									</span>
								</Table.Cell>

								<Table.Cell>
									<span
										class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium {kindBadgeClass(
											target.kind
										)}"
									>
										{target.kind === 'throttle' ? 'throttle' : 'daily quota'}
									</span>
								</Table.Cell>

								<Table.Cell class="text-right tabular-nums">{target.limit_count}</Table.Cell>

								<Table.Cell class="text-sm text-muted-foreground">
									{windowLabel(target.window_value, target.window_unit)}
								</Table.Cell>

								<Table.Cell>
									{#if toggling[target.key]}
										<Loader2 class="size-4 animate-spin text-muted-foreground" />
									{:else}
										<Switch
											checked={target.enabled}
											disabled={toggling[target.key]}
											onCheckedChange={() => handleToggleEnabled(target)}
										/>
									{/if}
								</Table.Cell>

								<Table.Cell class="text-right">
									<div class="flex justify-end gap-2">
										<Button variant="outline" size="sm" onclick={() => openEditDialog(target)}>
											Edit
										</Button>
										<Button
											variant="outline"
											size="sm"
											onclick={() => openResetDialog(target, 'reset')}
										>
											Reset
										</Button>
										{#if target.is_external}
											<Button
												variant="outline"
												size="sm"
												class="text-destructive hover:text-destructive"
												onclick={() => openDeleteDialog(target)}
											>
												Delete
											</Button>
										{:else}
											<Button
												variant="outline"
												size="sm"
												onclick={() => openResetDialog(target, 'reset-defaults')}
											>
												Defaults
											</Button>
										{/if}
									</div>
								</Table.Cell>
							</Table.Row>
						{/each}
					{/each}
				{/if}
			</Table.Body>
		</Table.Root>
	</div>
</div>

<!-- Edit limit/window -->
<Dialog.Root bind:open={editDialogOpen}>
	<Dialog.Content class="max-w-md">
		<Dialog.Header>
			<Dialog.Title>Edit {editingTarget?.name}</Dialog.Title>
			<Dialog.Description>
				Update this target's limit and window. Changes take effect immediately, without a
				restart.
			</Dialog.Description>
		</Dialog.Header>
		<form onsubmit={handleEditSubmit} class="space-y-4">
			<div class="space-y-2">
				<Label for="target-limit">Limit</Label>
				<Input id="target-limit" type="number" min="1" bind:value={formLimit} required />
			</div>
			<div class="grid grid-cols-2 gap-4">
				<div class="space-y-2">
					<Label for="target-window-value">Window value</Label>
					<Input
						id="target-window-value"
						type="number"
						min="1"
						bind:value={formWindowValue}
						required
					/>
				</div>
				<div class="space-y-2">
					<Label>Window unit</Label>
					<Select.Root
						type="single"
						value={formWindowUnit}
						onValueChange={(v) => (formWindowUnit = v as WindowUnit)}
					>
						<Select.Trigger class="w-full">
							{formWindowUnit}
						</Select.Trigger>
						<Select.Content>
							{#each WINDOW_UNITS as unit (unit)}
								<Select.Item value={unit} label={unit}>{unit}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
			</div>
			<Dialog.Footer>
				<Button
					type="button"
					variant="outline"
					onclick={() => (editDialogOpen = false)}
					disabled={saving}
				>
					Cancel
				</Button>
				<Button type="submit" disabled={saving}>
					{saving ? 'Saving...' : 'Save changes'}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>

<!-- Reset counters / reset to defaults confirmation -->
<AlertDialog.Root bind:open={resetDialogOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>
				{resetAction === 'reset' ? 'Reset counters?' : 'Reset to defaults?'}
			</AlertDialog.Title>
			<AlertDialog.Description>
				{#if resetAction === 'reset'}
					Clears today's usage count for <strong>{resetTarget?.name}</strong>. The limit and
					window are unchanged.
				{:else}
					Overwrites <strong>{resetTarget?.name}</strong>'s enabled/limit/window back to its
					built-in defaults, clearing any admin edit.
				{/if}
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel
				onclick={() => {
					resetDialogOpen = false;
					resetTarget = null;
				}}
				disabled={resetting}
			>
				Cancel
			</AlertDialog.Cancel>
			<AlertDialog.Action onclick={confirmReset} disabled={resetting}>
				{#if resetting}
					<Loader2 class="mr-2 size-4 animate-spin" />
				{/if}
				{resetAction === 'reset' ? 'Reset counters' : 'Reset to defaults'}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>

<!-- Create external target -->
<Dialog.Root bind:open={createDialogOpen}>
	<Dialog.Content class="max-w-lg">
		<Dialog.Header>
			<Dialog.Title>Create external target</Dialog.Title>
			<Dialog.Description>
				A DB-backed target another app checks via the API. Scope and kind are fixed at creation.
			</Dialog.Description>
		</Dialog.Header>
		<form onsubmit={handleCreateSubmit} class="space-y-4">
			<div class="space-y-2">
				<Label for="ext-key">Target key</Label>
				<Input id="ext-key" bind:value={cForm.key} placeholder="cashflow.entry.create" required />
			</div>
			<div class="grid grid-cols-2 gap-4">
				<div class="space-y-2">
					<Label for="ext-app">App</Label>
					<Input id="ext-app" bind:value={cForm.app} placeholder="defaults to key prefix" />
				</div>
				<div class="space-y-2">
					<Label for="ext-name">Name</Label>
					<Input id="ext-name" bind:value={cForm.name} placeholder="Entry create" />
				</div>
			</div>
			<div class="space-y-2">
				<Label for="ext-desc">Description</Label>
				<Input id="ext-desc" bind:value={cForm.description} />
			</div>
			<div class="grid grid-cols-2 gap-4">
				<div class="space-y-2">
					<Label>Scope</Label>
					<Select.Root type="single" value={cForm.scope} onValueChange={(v) => (cForm.scope = v as Scope)}>
						<Select.Trigger class="w-full">{cForm.scope}</Select.Trigger>
						<Select.Content>
							{#each SCOPES as s (s)}
								<Select.Item value={s} label={s}>{s}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
				<div class="space-y-2">
					<Label>Kind</Label>
					<Select.Root type="single" value={cForm.kind} onValueChange={(v) => (cForm.kind = v as Kind)}>
						<Select.Trigger class="w-full">{cForm.kind === 'throttle' ? 'throttle' : 'daily quota'}</Select.Trigger>
						<Select.Content>
							{#each KINDS as k (k)}
								<Select.Item value={k} label={k}>{k === 'throttle' ? 'throttle' : 'daily quota'}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
			</div>
			<div class="grid grid-cols-3 gap-4">
				<div class="space-y-2">
					<Label for="ext-limit">Limit</Label>
					<Input id="ext-limit" type="number" min="1" bind:value={cForm.limit_count} required />
				</div>
				<div class="space-y-2">
					<Label for="ext-wv">Window value</Label>
					<Input id="ext-wv" type="number" min="1" bind:value={cForm.window_value} required />
				</div>
				<div class="space-y-2">
					<Label>Window unit</Label>
					<Select.Root type="single" value={cForm.window_unit} onValueChange={(v) => (cForm.window_unit = v as WindowUnit)}>
						<Select.Trigger class="w-full">{cForm.window_unit}</Select.Trigger>
						<Select.Content>
							{#each WINDOW_UNITS as unit (unit)}
								<Select.Item value={unit} label={unit}>{unit}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
			</div>
			<Dialog.Footer>
				<Button type="button" variant="outline" onclick={() => (createDialogOpen = false)}>Cancel</Button>
				<Button type="submit" disabled={creating}>
					{#if creating}<Loader2 class="mr-2 size-4 animate-spin" />{/if}
					Create
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>

<!-- Delete external target -->
<AlertDialog.Root bind:open={deleteDialogOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete this external target?</AlertDialog.Title>
			<AlertDialog.Description>
				{deleteTarget?.key} will stop enforcing immediately. This cannot be undone.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel
				onclick={() => {
					deleteDialogOpen = false;
					deleteTarget = null;
				}}
				disabled={deleting}
			>
				Cancel
			</AlertDialog.Cancel>
			<AlertDialog.Action onclick={confirmDelete} disabled={deleting}>
				{#if deleting}<Loader2 class="mr-2 size-4 animate-spin" />{/if}
				Delete
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
