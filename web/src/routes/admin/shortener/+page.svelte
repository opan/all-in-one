<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button/index';
	import { Switch } from '$lib/components/ui/switch/index';
	import * as AlertDialog from '$lib/components/ui/alert-dialog/index';
	import * as Table from '$lib/components/ui/table/index';
	import { toast, Toaster } from 'svelte-sonner';
	import { Trash2, Loader2 } from '@lucide/svelte/icons';
	import {
		listAllShortLinks,
		adminSetShortLinkActive,
		adminDeleteShortLink,
		type AdminShortLink
	} from '$lib/shortener-admin-api';

	const PAGE_SIZE = 20;

	let links = $state<AdminShortLink[]>([]);
	let total = $state(0);
	let page = $state(1);
	let loading = $state(true);
	let error = $state('');

	let totalPages = $derived(Math.max(1, Math.ceil(total / PAGE_SIZE)));

	// Delete confirmation
	let deleteOpen = $state(false);
	let linkToDelete = $state<AdminShortLink | null>(null);
	let deleting = $state(false);

	// Per-row toggle loading
	let toggling = $state<Record<string, boolean>>({});

	onMount(() => load(1));

	async function load(targetPage: number) {
		loading = true;
		error = '';
		try {
			const res = await listAllShortLinks(targetPage, PAGE_SIZE);
			links = res.links;
			total = res.total;
			page = res.page;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load short links';
		} finally {
			loading = false;
		}
	}

	function prevPage() {
		if (page > 1) load(page - 1);
	}

	function nextPage() {
		if (page < totalPages) load(page + 1);
	}

	async function handleToggleActive(link: AdminShortLink) {
		toggling = { ...toggling, [link.code]: true };
		try {
			await adminSetShortLinkActive(link.code, !link.is_active);
			links = links.map((l) => (l.code === link.code ? { ...l, is_active: !l.is_active } : l));
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to update short link');
		} finally {
			toggling = { ...toggling, [link.code]: false };
		}
	}

	function confirmDelete(link: AdminShortLink) {
		linkToDelete = link;
		deleteOpen = true;
	}

	async function handleDelete() {
		if (!linkToDelete) return;
		deleting = true;
		try {
			const code = linkToDelete.code;
			await adminDeleteShortLink(code);
			toast.success(`${code} deleted`);
			deleteOpen = false;
			linkToDelete = null;
			// Refetch rather than filter locally: deleting the last row on a page
			// (other than page 1) should fall back to the previous page instead
			// of showing an empty page, and total must reflect the server.
			await load(links.length === 1 && page > 1 ? page - 1 : page);
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to delete short link');
		} finally {
			deleting = false;
		}
	}

	function truncate(str: string, n: number): string {
		return str.length > n ? str.slice(0, n) + '…' : str;
	}
</script>

<Toaster richColors position="top-center" />

<div class="container mx-auto p-6 space-y-6">
	<div class="space-y-2">
		<h1 class="text-3xl font-bold tracking-tight">Shortener</h1>
		<p class="text-muted-foreground">
			{total} {total === 1 ? 'link' : 'links'} across all users. Activate, deactivate, or delete any link.
		</p>
	</div>

	{#if error}
		<div class="rounded-md bg-destructive/15 p-3 text-sm text-destructive">{error}</div>
	{/if}

	<div class="rounded-md border">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head class="w-32">Short code</Table.Head>
					<Table.Head>Target URL</Table.Head>
					<Table.Head class="w-32">Owner</Table.Head>
					<Table.Head class="w-20 text-right">Clicks</Table.Head>
					<Table.Head class="w-20">Status</Table.Head>
					<Table.Head class="w-24 text-right">Actions</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#if !loading && links.length === 0}
					<Table.Row>
						<Table.Cell colspan={6} class="h-24 text-center text-muted-foreground">
							{#if error}
								No short links to show.
							{:else}
								No short links yet.
							{/if}
						</Table.Cell>
					</Table.Row>
				{:else}
					{#each links as link (link.code)}
						<Table.Row class={link.is_active ? '' : 'opacity-50'}>
							<Table.Cell class="font-mono text-sm">{link.code}</Table.Cell>

							<Table.Cell class="max-w-xs">
								<a
									href={link.target_url}
									target="_blank"
									rel="noopener noreferrer"
									class="text-sm text-muted-foreground hover:text-foreground hover:underline"
									title={link.target_url}
								>
									{truncate(link.target_url, 60)}
								</a>
							</Table.Cell>

							<Table.Cell class="text-sm text-muted-foreground">
								{link.owner_username || '—'}
							</Table.Cell>

							<Table.Cell class="text-right text-sm tabular-nums">
								{link.click_count}
							</Table.Cell>

							<Table.Cell>
								<span
									class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium
									{link.is_active
										? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
										: 'bg-muted text-muted-foreground'}"
								>
									{link.is_active ? 'Active' : 'Inactive'}
								</span>
							</Table.Cell>

							<Table.Cell class="text-right">
								<div class="flex items-center justify-end gap-2">
									{#if toggling[link.code]}
										<Loader2 class="size-4 animate-spin text-muted-foreground" />
									{:else}
										<Switch
											checked={link.is_active}
											disabled={toggling[link.code]}
											onCheckedChange={() => handleToggleActive(link)}
										/>
									{/if}
									<Button
										variant="ghost"
										size="icon"
										class="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
										onclick={() => confirmDelete(link)}
										title="Delete link"
									>
										<Trash2 class="size-4" />
									</Button>
								</div>
							</Table.Cell>
						</Table.Row>
					{/each}
				{/if}
			</Table.Body>
		</Table.Root>
	</div>

	<div class="flex items-center justify-between">
		<p class="text-sm text-muted-foreground">
			{#if total > 0}
				Showing {(page - 1) * PAGE_SIZE + 1}–{Math.min(page * PAGE_SIZE, total)} of {total}
			{:else}
				0 results
			{/if}
		</p>
		<div class="flex items-center gap-2">
			<span class="text-sm text-muted-foreground">Page {page} of {totalPages}</span>
			<Button variant="outline" size="sm" onclick={prevPage} disabled={loading || page <= 1}>
				Previous
			</Button>
			<Button variant="outline" size="sm" onclick={nextPage} disabled={loading || page >= totalPages}>
				Next
			</Button>
		</div>
	</div>
</div>

<!-- Delete confirmation -->
<AlertDialog.Root bind:open={deleteOpen}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete short link?</AlertDialog.Title>
			<AlertDialog.Description>
				The short link <strong class="font-mono">{linkToDelete?.code}</strong> will be permanently
				deleted. Anyone following it will get a 404.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel
				onclick={() => {
					deleteOpen = false;
					linkToDelete = null;
				}}
				disabled={deleting}
			>
				Cancel
			</AlertDialog.Cancel>
			<AlertDialog.Action
				onclick={handleDelete}
				disabled={deleting}
				class="bg-destructive text-destructive-foreground hover:bg-destructive/90"
			>
				{#if deleting}
					<Loader2 class="mr-2 size-4 animate-spin" />
				{/if}
				Delete
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
