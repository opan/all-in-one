<script lang="ts">
	import AuthShell from '../../components/auth-shell.svelte';

	let { children } = $props();
</script>

<!-- Password recovery and email confirmation (RFC-003), presented as the app
     the user came from when there is one. -->
<AuthShell>
	{@render children?.()}
</AuthShell>
