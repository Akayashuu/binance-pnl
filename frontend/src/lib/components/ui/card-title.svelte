<script lang="ts">
	import { cn } from '$lib/utils';

	interface Props {
		class?: string;
		children?: import('svelte').Snippet;
	}

	let { class: className, children }: Props = $props();
</script>

<h3 class={cn('text-sm font-medium leading-none tracking-tight text-muted-foreground', className)}>
	{@render children?.()}
</h3>
