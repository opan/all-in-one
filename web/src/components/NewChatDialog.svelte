<script lang="ts">
	import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "$lib/components/ui/dialog";
	import { Button } from "$lib/components/ui/button";
	import { Input } from "$lib/components/ui/input";
	import { searchUsers, sendInvite, type User } from "$lib/chat-api";
	import { Skeleton } from "$lib/components/ui/skeleton";

	interface Props {
		open?: boolean;
		onOpenChange?: (open: boolean) => void;
		/** Called when invites have been sent (kept as onSessionCreated for backward compat) */
		onSessionCreated?: () => void;
	}

	let { open = $bindable(false), onOpenChange, onSessionCreated }: Props = $props();

	let searchQuery = $state("");
	let searchResults = $state<User[]>([]);
	let selectedUsers = $state<Set<string>>(new Set());
	let searching = $state(false);
	let sending = $state(false);
	let error = $state<string | null>(null);
	let successMessage = $state<string | null>(null);

	// Debounce search
	let searchTimeout: ReturnType<typeof setTimeout> | null = null;

	function handleSearchInput(event: Event) {
		const target = event.target as HTMLInputElement;
		searchQuery = target.value;
		
		if (searchTimeout) {
			clearTimeout(searchTimeout);
		}

		if (searchQuery.trim().length < 2) {
			searchResults = [];
			return;
		}

		searchTimeout = setTimeout(async () => {
			await performSearch();
		}, 300);
	}

	async function performSearch() {
		if (searchQuery.trim().length < 2) {
			searchResults = [];
			return;
		}

		searching = true;
		error = null;

		try {
			searchResults = await searchUsers(searchQuery);
		} catch (err) {
			error = err instanceof Error ? err.message : "Failed to search users";
			searchResults = [];
		} finally {
			searching = false;
		}
	}

	function toggleUserSelection(userId: string) {
		const newSelected = new Set(selectedUsers);
		if (newSelected.has(userId)) {
			newSelected.delete(userId);
		} else {
			newSelected.add(userId);
		}
		selectedUsers = newSelected;
	}

	async function handleSendInvite() {
		if (selectedUsers.size === 0) {
			error = "Please select at least one user";
			return;
		}

		sending = true;
		error = null;
		successMessage = null;

		try {
			await sendInvite({ participants: Array.from(selectedUsers) });

			const count = selectedUsers.size;
			successMessage = count === 1
				? "Invite sent! The chat will start when they accept."
				: `${count} invites sent! The chat will start when they accept.`;

			// Reset selection and search
			searchQuery = "";
			searchResults = [];
			selectedUsers = new Set();

			if (onSessionCreated) {
				onSessionCreated();
			}

			// Auto-close after brief confirmation
			setTimeout(() => {
				open = false;
				successMessage = null;
				if (onOpenChange) onOpenChange(false);
			}, 1800);
		} catch (err) {
			error = err instanceof Error ? err.message : "Failed to send invite";
		} finally {
			sending = false;
		}
	}

	function handleOpenChange(newOpen: boolean) {
		open = newOpen;
		if (onOpenChange) {
			onOpenChange(newOpen);
		}

		if (!newOpen) {
			searchQuery = "";
			searchResults = [];
			selectedUsers = new Set();
			error = null;
			successMessage = null;
		}
	}
</script>

<Dialog open={open} onOpenChange={handleOpenChange}>
	<DialogContent class="sm:max-w-[500px]">
		<DialogHeader>
			<DialogTitle>Start New Chat</DialogTitle>
			<DialogDescription>
				Search for users to invite — the chat starts when they accept
			</DialogDescription>
		</DialogHeader>

		<div class="space-y-4 py-4">
			<!-- Search Input -->
			<div class="space-y-2">
				<Input
					type="text"
					placeholder="Search users by name, username, or email..."
					value={searchQuery}
					oninput={handleSearchInput}
					disabled={sending}
				/>
			</div>

			<!-- Selected Users -->
			{#if selectedUsers.size > 0}
				<div class="space-y-2">
					<div class="text-sm font-medium">Selected ({selectedUsers.size}):</div>
					<div class="flex flex-wrap gap-2">
						{#each Array.from(selectedUsers) as userId}
							{@const user = searchResults.find(u => u.id === userId)}
							{#if user}
								<div class="flex items-center gap-1 rounded-md bg-primary/10 px-2 py-1 text-sm">
									<span>{user.username}</span>
									<button
										onclick={() => toggleUserSelection(userId)}
										class="ml-1 rounded hover:bg-primary/20"
										disabled={sending}
										aria-label="Remove {user.username}"
									>
										<svg class="h-3 w-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
										</svg>
									</button>
								</div>
							{/if}
						{/each}
					</div>
				</div>
			{/if}

			<!-- Search Results -->
			<div class="max-h-[300px] space-y-2 overflow-y-auto">
				{#if searching}
					<div class="space-y-2">
						<Skeleton class="h-12 w-full" />
						<Skeleton class="h-12 w-full" />
						<Skeleton class="h-12 w-full" />
					</div>
				{:else if searchResults.length > 0}
					{#each searchResults as user}
						<button
							class="flex w-full items-center justify-between rounded-md border p-3 text-left transition-colors hover:bg-accent"
							onclick={() => toggleUserSelection(user.id)}
							disabled={sending}
						>
							<div class="flex-1">
								<div class="font-medium">{user.username}</div>
								<div class="text-sm text-muted-foreground">{user.name}</div>
								<div class="text-xs text-muted-foreground">{user.email}</div>
							</div>
							{#if selectedUsers.has(user.id)}
								<div class="flex h-5 w-5 items-center justify-center rounded-full bg-primary text-primary-foreground">
									<svg class="h-3 w-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
									</svg>
								</div>
							{:else}
								<div class="h-5 w-5 rounded-full border-2"></div>
							{/if}
						</button>
					{/each}
				{:else if searchQuery.trim().length >= 2}
					<div class="py-8 text-center text-sm text-muted-foreground">
						No users found
					</div>
				{:else if searchQuery.trim().length > 0}
					<div class="py-8 text-center text-sm text-muted-foreground">
						Type at least 2 characters to search
					</div>
				{/if}
			</div>

			<!-- Success Message -->
			{#if successMessage}
				<div class="rounded-md bg-green-50 p-3 text-sm text-green-700 dark:bg-green-950 dark:text-green-300">
					{successMessage}
				</div>
			{/if}

			<!-- Error Message -->
			{#if error}
				<div class="rounded-md bg-destructive/10 p-3 text-sm text-destructive">
					{error}
				</div>
			{/if}
		</div>

		<!-- Actions -->
		<div class="flex justify-end gap-2">
			<Button variant="outline" onclick={() => handleOpenChange(false)} disabled={sending}>
				Cancel
			</Button>
			<Button
				onclick={handleSendInvite}
				disabled={selectedUsers.size === 0 || sending}
			>
				{sending ? "Sending..." : `Send Invite${selectedUsers.size > 1 ? 's' : ''} (${selectedUsers.size})`}
			</Button>
		</div>
	</DialogContent>
</Dialog>
