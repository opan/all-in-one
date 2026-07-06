<script lang="ts">
	import * as Tabs from '$lib/components/ui/tabs';
	import FeaturesTab from './FeaturesTab.svelte';
	import GroupsTab from './GroupsTab.svelte';
	import UsersTab from './UsersTab.svelte';

	// bits-ui's Tabs.Content keeps every panel mounted (it never
	// unmounts inactive tabs), so each tab's onMount only ever fires once.
	// Without this, e.g. a group created in the Groups tab wouldn't show up
	// in the Users tab's reassignment dropdown until a full page reload.
	// Passing `active` lets each tab re-fetch on every switch instead.
	let activeTab = $state('users');
</script>

<div class="space-y-6">
	<div class="space-y-2">
		<h2 class="text-2xl font-semibold">Access Management</h2>
		<p class="text-sm text-muted-foreground">
			Control which app-features users can access, via groups (presets) and per-user overrides.
		</p>
	</div>

	<Tabs.Root bind:value={activeTab}>
		<Tabs.List>
			<Tabs.Trigger value="users">Users</Tabs.Trigger>
			<Tabs.Trigger value="groups">Groups</Tabs.Trigger>
			<Tabs.Trigger value="features">Features</Tabs.Trigger>
		</Tabs.List>
		<Tabs.Content value="users" class="pt-4">
			<UsersTab active={activeTab === 'users'} />
		</Tabs.Content>
		<Tabs.Content value="groups" class="pt-4">
			<GroupsTab active={activeTab === 'groups'} />
		</Tabs.Content>
		<Tabs.Content value="features" class="pt-4">
			<FeaturesTab active={activeTab === 'features'} />
		</Tabs.Content>
	</Tabs.Root>
</div>
