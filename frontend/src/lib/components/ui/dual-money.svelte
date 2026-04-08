<script lang="ts">
	import { formatDualMoney } from '$lib/utils/format';
	import type { Money } from '$lib/api/types';

	type Props = {
		money: Money;
		// Optional Tailwind classes applied to the whole container — colours,
		// font weight, etc. The secondary line inherits a muted variant.
		class?: string;
	};

	let { money, class: klass = '' }: Props = $props();
	let formatted = $derived(formatDualMoney(money));
</script>

<span class={`inline-flex flex-col items-end leading-tight ${klass}`}>
	<span class="tabular-nums">{formatted.primary}</span>
	{#if formatted.secondary}
		<span class="text-[10px] text-muted-foreground tabular-nums">{formatted.secondary}</span>
	{/if}
</span>
