<script lang="ts">
	import { onMount } from 'svelte';
	import { Switch } from '$lib/components/ui/switch/index';
	import * as Table from '$lib/components/ui/table/index';
	import { toast, Toaster } from 'svelte-sonner';
	import { Loader2 } from '@lucide/svelte/icons';
	import { listTargets, updateTarget, type RateLimitTarget } from '$lib/ratelimit-api';

	let targets = $state<RateLimitTarget[]>([]);
	let loading = $state(true);
	let error = $state('');

	// Per-row toggle loading
	let toggling = $state<Record<string, boolean>>({});

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
	<div class="space-y-2">
		<h1 class="text-3xl font-bold tracking-tight">Rate Limits</h1>
		<p class="text-muted-foreground">
			{targets.length} rate-limited {targets.length === 1 ? 'target' : 'targets'}. Toggle,
			edit, or reset any target's limit.
		</p>
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
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#if !loading && targets.length === 0}
					<Table.Row>
						<Table.Cell colspan={6} class="h-24 text-center text-muted-foreground">
							{#if error}
								No rate limit targets to show.
							{:else}
								No rate limit targets configured.
							{/if}
						</Table.Cell>
					</Table.Row>
				{:else}
					{#each targets as target (target.key)}
						<Table.Row class={target.enabled ? '' : 'opacity-50'}>
							<Table.Cell>
								<div class="flex flex-col">
									<span class="font-medium">{target.name}</span>
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
						</Table.Row>
					{/each}
				{/if}
			</Table.Body>
		</Table.Root>
	</div>
</div>
