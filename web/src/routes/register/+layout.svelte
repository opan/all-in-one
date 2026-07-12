<script lang="ts">
	import '../../app.css';
	import ThemeToggle from "../../components/theme-toggle.svelte";

	let { children } = $props();
</script>

<div class="min-h-screen bg-background text-foreground">
	<header class="flex h-14 shrink-0 items-center gap-2 border-b px-4">
		<div class="flex-1">
			<a href="/" class="text-lg font-semibold">All-in-one</a>
		</div>

		<div class="ml-auto flex items-center gap-2">
			<ThemeToggle />
		</div>
	</header>

	<main class="flex-1">
		{@render children?.()}
	</main>
</div>
