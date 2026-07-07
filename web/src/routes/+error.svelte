<script lang="ts">
	import { page } from '$app/stores';
	import { Button } from '$lib/components/ui/button';
	import ShieldBanIcon from '@lucide/svelte/icons/shield-ban';
	import FileQuestionMarkIcon from '@lucide/svelte/icons/file-question-mark';
	import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';

	const status = $derived($page.status);
	const message = $derived($page.error?.message || 'Something went wrong.');

	const presentation = $derived(
		status === 403
			? { icon: ShieldBanIcon, title: 'Access denied' }
			: status === 404
				? { icon: FileQuestionMarkIcon, title: 'Page not found' }
				: { icon: TriangleAlertIcon, title: 'Something went wrong' }
	);
</script>

<div class="flex min-h-[60vh] flex-col items-center justify-center gap-4 px-4 text-center">
	<presentation.icon class="text-muted-foreground size-12" />
	<div class="space-y-1">
		<h1 class="text-2xl font-semibold">{presentation.title}</h1>
		<p class="text-muted-foreground">{message}</p>
	</div>
	<p class="text-muted-foreground text-sm">Error {status}</p>
	<Button href="/">Go back home</Button>
</div>
