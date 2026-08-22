<script lang="ts">
	import * as Sidebar from "$lib/components/ui/sidebar/index";
	import * as Breadcrumb from "$lib/components/ui/breadcrumb/index";
	import { Separator } from "$lib/components/ui/separator/index";
	import ThemeToggle from "./theme-toggle.svelte";
	import AppSidebar from "./app-sidebar.svelte";

	interface Props {
		children?: import('svelte').Snippet;
		breadcrumbs?: Array<{ label: string; href?: string }>;
		currentPath: string;
	}

	let { children, breadcrumbs = [{ label: 'Home', href: '/home' }], currentPath }: Props = $props();
</script>

<Sidebar.Provider>
	<AppSidebar {currentPath} />
	
	<Sidebar.Inset>
		<header class="flex h-14 shrink-0 items-center gap-2 border-b px-4">
			<Sidebar.Trigger class="-ml-1" />
			<Separator orientation="vertical" class="mr-2 h-4" />
			
			<Breadcrumb.Root>
				<Breadcrumb.List>
					{#each breadcrumbs as crumb, i (i)}
						{#if i > 0}
							<Breadcrumb.Separator />
						{/if}
						<Breadcrumb.Item>
							{#if crumb.href && i < breadcrumbs.length - 1}
								<Breadcrumb.Link href={crumb.href}>{crumb.label}</Breadcrumb.Link>
							{:else}
								<Breadcrumb.Page>{crumb.label}</Breadcrumb.Page>
							{/if}
						</Breadcrumb.Item>
					{/each}
				</Breadcrumb.List>
			</Breadcrumb.Root>
			
			<div class="ml-auto flex items-center gap-2">
				<ThemeToggle />
			</div>
		</header>

		<main class="flex-1 overflow-auto">
			{@render children?.()}
		</main>
	</Sidebar.Inset>
</Sidebar.Provider>
