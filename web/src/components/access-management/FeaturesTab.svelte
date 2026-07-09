<script lang="ts">
	import * as Table from '$lib/components/ui/table';
	import { listFeatures, type Feature } from '$lib/rbac-api';

	interface Props {
		active: boolean;
	}
	let { active }: Props = $props();

	let features = $state<Feature[]>([]);
	let loading = $state(true);
	let error = $state('');

	// Re-fetch every time this tab becomes active, not just on first mount —
	// bits-ui's Tabs.Content never unmounts inactive panels, so onMount alone
	// would only ever run once for this component's whole lifetime.
	$effect(() => {
		if (active) load();
	});

	async function load() {
		loading = true;
		error = '';
		try {
			features = await listFeatures();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load features';
		} finally {
			loading = false;
		}
	}
</script>

<div class="space-y-4">
	<p class="text-sm text-muted-foreground">
		Features are code-defined (they map to real apps) and cannot be created or edited here — new
		apps register themselves automatically.
	</p>

	{#if error}
		<div class="rounded-md bg-destructive/15 p-3 text-sm text-destructive">{error}</div>
	{/if}

	<div class="rounded-md border">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head>Key</Table.Head>
					<Table.Head>Name</Table.Head>
					<Table.Head>Description</Table.Head>
					<Table.Head>Access</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#if loading}
					<Table.Row>
						<Table.Cell colspan={4} class="h-24 text-center text-muted-foreground">Loading...</Table.Cell>
					</Table.Row>
				{:else if features.length === 0}
					<Table.Row>
						<Table.Cell colspan={4} class="h-24 text-center text-muted-foreground">No features found.</Table.Cell>
					</Table.Row>
				{:else}
					{#each features as feature (feature.id)}
						<Table.Row>
							<Table.Cell class="font-mono text-xs">{feature.key}</Table.Cell>
							<Table.Cell>{feature.name}</Table.Cell>
							<Table.Cell class="text-muted-foreground">{feature.description || '—'}</Table.Cell>
							<Table.Cell>
								{#if feature.admin_only}
									<span class="inline-flex items-center rounded-full bg-amber-100 px-2.5 py-0.5 text-xs font-medium text-amber-800 dark:bg-amber-900 dark:text-amber-200">
										Admin only
									</span>
								{:else}
									<span class="inline-flex items-center rounded-full bg-green-100 px-2.5 py-0.5 text-xs font-medium text-green-800 dark:bg-green-900 dark:text-green-200">
										Regular users (default)
									</span>
								{/if}
							</Table.Cell>
						</Table.Row>
					{/each}
				{/if}
			</Table.Body>
		</Table.Root>
	</div>
</div>
