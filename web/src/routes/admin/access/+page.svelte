<script lang="ts">
	import * as Tabs from '$lib/components/ui/tabs';
	import GroupsTab from '../../../components/access-management/GroupsTab.svelte';
	import FeaturesTab from '../../../components/access-management/FeaturesTab.svelte';

	// bits-ui's Tabs.Content keeps every panel mounted (it never unmounts inactive
	// tabs), so each tab's data load must be driven by an `active` prop + $effect
	// rather than onMount (which would only ever fire once). See each tab component.
	let activeTab = $state('groups');
</script>

<div class="container mx-auto p-6 space-y-6">
	<div class="space-y-2">
		<h1 class="text-3xl font-bold tracking-tight">Access</h1>
		<p class="text-muted-foreground">
			Control which app-features users can access, via groups (presets) and per-user overrides.
			Assign groups and overrides per person on the <a href="/admin/users" class="underline">Users</a> page.
		</p>
	</div>

	<Tabs.Root bind:value={activeTab}>
		<Tabs.List>
			<Tabs.Trigger value="groups">Groups</Tabs.Trigger>
			<Tabs.Trigger value="features">Features</Tabs.Trigger>
		</Tabs.List>
		<Tabs.Content value="groups" class="pt-4">
			<GroupsTab active={activeTab === 'groups'} />
		</Tabs.Content>
		<Tabs.Content value="features" class="pt-4">
			<FeaturesTab active={activeTab === 'features'} />
		</Tabs.Content>
	</Tabs.Root>
</div>
