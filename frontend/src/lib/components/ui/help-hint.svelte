<script lang="ts">
	import { HelpCircle } from 'lucide-svelte';

	type Props = { text: string };
	let { text }: Props = $props();
</script>

<span
	class="inline-flex cursor-help align-middle text-muted-foreground/60 hover:text-muted-foreground"
	title={text}
	aria-label={text}
>
	<HelpCircle class="h-3 w-3" />
</span>
