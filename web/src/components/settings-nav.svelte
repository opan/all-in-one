<script lang="ts">
	interface NavItem {
		id: string;
		label: string;
		icon: string;
	}

	interface Props {
		items: NavItem[];
		activeSection: string;
		onSelect: (id: string) => void;
	}

	let { items, activeSection, onSelect }: Props = $props();
</script>

<nav class="w-full md:w-56 space-y-1">
	{#each items as item}
		<button
			type="button"
			onclick={() => onSelect(item.id)}
			class="flex items-center gap-3 w-full px-3 py-2 text-sm rounded-md transition-colors text-left {activeSection === item.id 
				? 'bg-secondary text-secondary-foreground font-medium' 
				: 'text-muted-foreground hover:bg-secondary/50 hover:text-foreground'}"
		>
			<span class="text-lg">{item.icon}</span>
			<span>{item.label}</span>
		</button>
	{/each}
</nav>
