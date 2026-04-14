<script lang="ts">
	import { _ } from 'svelte-i18n';
	import type { Source } from '$lib/api/types';

	const STYLES: Record<Source, string> = {
		spot: 'bg-blue-500/20 text-blue-300',
		convert: 'bg-purple-500/20 text-purple-300',
		fiat_buy: 'bg-emerald-500/20 text-emerald-300',
		recurring: 'bg-cyan-500/20 text-cyan-300',
		deposit: 'bg-amber-500/20 text-amber-300',
		earn_reward: 'bg-pink-500/20 text-pink-300'
	};

	interface Props {
		source: Source;
	}

	let { source }: Props = $props();
</script>

<span
	class={`rounded px-2 py-0.5 text-xs font-semibold ${STYLES[source] ?? 'bg-muted text-muted-foreground'}`}
>
	{$_(`sources.${source}`)}
</span>
