<script lang="ts">
	import { onMount } from 'svelte';
	import type { Component } from 'svelte';
	import type { IconProps } from '@lucide/svelte';
	import {
		Table,
		MessageSquare,
		Link2,
		Users,
		ShieldCheck,
		Gauge,
		Mail,
		ArrowRight
	} from '@lucide/svelte/icons';
	import * as Card from '$lib/components/ui/card/index';
	import { auth, loadAuth } from '$lib/stores/auth';
	import type { DashboardSummary } from '$lib/dashboard-api';

	let { data }: { data: { summary: DashboardSummary } } = $props();

	const summary = $derived(data.summary);

	onMount(() => {
		// The sidebar also populates this store; load here too so a direct hit on
		// "/" shows the greeting without waiting on the sidebar's mount.
		if (!$auth) loadAuth();
	});

	const greetingName = $derived($auth?.name || $auth?.username || '');

	type StatTile = {
		label: string;
		value: number;
		icon: Component<IconProps, {}, ''>;
		href: string;
	};

	// Both the stat tiles and the launcher cards are driven by which sections the
	// summary contains — the backend includes a section only for features the
	// user can access, so this stays in lock-step with server-side RBAC.
	const tiles = $derived.by<StatTile[]>(() => {
		const out: StatTile[] = [];
		if (summary.listing) {
			out.push({ label: 'Topics', value: summary.listing.topics, icon: Table, href: '/listing/topics' });
		}
		if (summary.chat) {
			out.push({ label: 'Conversations', value: summary.chat.conversations, icon: MessageSquare, href: '/chat' });
			out.push({ label: 'Pending invites', value: summary.chat.pending_invites, icon: Mail, href: '/chat' });
		}
		if (summary.shortener) {
			out.push({ label: 'Short links', value: summary.shortener.links, icon: Link2, href: '/shortener' });
		}
		return out;
	});

	type AppCard = {
		key: keyof DashboardSummary;
		title: string;
		description: string;
		href: string;
		icon: Component<IconProps, {}, ''>;
	};

	const allApps: AppCard[] = [
		{ key: 'listing', title: 'Listings', description: 'Create and manage your item listings.', href: '/listing/topics', icon: Table },
		{ key: 'chat', title: 'Chats', description: 'Real-time conversations and invites.', href: '/chat', icon: MessageSquare },
		{ key: 'shortener', title: 'Shortener', description: 'Create and track short links.', href: '/shortener', icon: Link2 }
	];

	const apps = $derived(allApps.filter((a) => summary[a.key] !== undefined));

	const isAdmin = $derived($auth?.is_admin === true);

	const adminLinks: Array<{ title: string; href: string; icon: Component<IconProps, {}, ''> }> = [
		{ title: 'Users', href: '/admin/users', icon: Users },
		{ title: 'Access', href: '/admin/access', icon: ShieldCheck },
		{ title: 'Shortener', href: '/admin/shortener', icon: Link2 },
		{ title: 'Rate Limits', href: '/admin/ratelimit', icon: Gauge }
	];
</script>

<div class="container mx-auto p-6">
	<div class="mx-auto max-w-5xl space-y-8">
		<!-- Greeting -->
		<header>
			<h1 class="text-3xl font-bold tracking-tight">
				Welcome back{greetingName ? `, ${greetingName}` : ''}
			</h1>
			<p class="text-muted-foreground mt-1">Here's what's happening across your apps.</p>
		</header>

		<!-- Stat tiles -->
		{#if tiles.length > 0}
			<section class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
				{#each tiles as tile}
					<a href={tile.href} class="group">
						<Card.Root class="transition-colors group-hover:border-primary/50">
							<Card.Content class="flex items-center gap-3 p-4">
								<div class="bg-muted text-muted-foreground flex size-10 shrink-0 items-center justify-center rounded-lg">
									<tile.icon class="size-5" />
								</div>
								<div class="min-w-0">
									<div class="text-2xl font-semibold leading-none tabular-nums">{tile.value}</div>
									<div class="text-muted-foreground mt-1 truncate text-xs">{tile.label}</div>
								</div>
							</Card.Content>
						</Card.Root>
					</a>
				{/each}
			</section>
		{/if}

		<!-- App launcher -->
		<section class="space-y-3">
			<h2 class="text-muted-foreground text-sm font-medium">Your apps</h2>
			{#if apps.length > 0}
				<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
					{#each apps as app}
						<a href={app.href} class="group">
							<Card.Root class="h-full transition-colors group-hover:border-primary/50">
								<Card.Header>
									<div class="flex items-center justify-between">
										<div class="bg-primary/10 text-primary flex size-10 items-center justify-center rounded-lg">
											<app.icon class="size-5" />
										</div>
										<ArrowRight class="text-muted-foreground size-4 transition-transform group-hover:translate-x-0.5" />
									</div>
									<Card.Title class="mt-3">{app.title}</Card.Title>
									<Card.Description>{app.description}</Card.Description>
								</Card.Header>
							</Card.Root>
						</a>
					{/each}
				</div>
			{:else}
				<Card.Root>
					<Card.Content class="text-muted-foreground p-6 text-sm">
						You don't have access to any apps yet. Ask an administrator to grant you access.
					</Card.Content>
				</Card.Root>
			{/if}
		</section>

		<!-- Admin quick links -->
		{#if isAdmin}
			<section class="space-y-3">
				<h2 class="text-muted-foreground text-sm font-medium">Admin</h2>
				<div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
					{#each adminLinks as link}
						<a
							href={link.href}
							class="hover:border-primary/50 hover:bg-accent flex items-center gap-2 rounded-lg border p-3 text-sm transition-colors"
						>
							<link.icon class="text-muted-foreground size-4 shrink-0" />
							<span class="truncate">{link.title}</span>
						</a>
					{/each}
				</div>
			</section>
		{/if}
	</div>
</div>
