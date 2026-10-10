<script lang="ts">
	import AuthShell from '../../components/auth-shell.svelte';

	let { children } = $props();
</script>

<AuthShell>
	{@render children?.()}
</AuthShell>
