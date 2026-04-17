<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { DualMoney } from '$lib/components/ui';
	import { formatPctBP, formatQuantity, pnlClass } from '$lib/utils/format';
	import type { PnL } from '$lib/api/types';

	const DUST_KEY = 'hideDust';
	const DUST_THRESHOLD = 0.1;

	interface Props {
		positions: PnL[];
	}

	let { positions }: Props = $props();
	let hideDust = $state(typeof localStorage !== 'undefined' && localStorage.getItem(DUST_KEY) === '1');

	function toggleDust() {
		hideDust = !hideDust;
		localStorage.setItem(DUST_KEY, hideDust ? '1' : '0');
	}

	function marketValue(pos: PnL): number {
		return Number(pos.market_value.display?.amount ?? pos.market_value.amount);
	}

	let filtered = $derived(
		hideDust ? positions.filter((p) => marketValue(p) >= DUST_THRESHOLD) : positions
	);
</script>

{#if positions.length === 0}
	<p class="mt-4 text-sm text-muted-foreground">{$_('dashboard.empty')}</p>
{:else}
	<div class="mt-4 flex items-center justify-end gap-2">
		<label class="flex cursor-pointer items-center gap-2 text-xs text-muted-foreground">
			<button
				role="switch"
				aria-checked={hideDust}
				aria-label={$_('dashboard.hide_dust')}
				onclick={toggleDust}
				class="relative inline-flex h-5 w-9 items-center rounded-full transition-colors {hideDust ? 'bg-primary' : 'bg-muted'}"
			>
				<span
					class="inline-block h-3.5 w-3.5 rounded-full bg-white transition-transform {hideDust ? 'translate-x-4.5' : 'translate-x-0.5'}"
				></span>
			</button>
			{$_('dashboard.hide_dust')}
		</label>
	</div>

	<div class="mt-2 overflow-hidden rounded-lg border bg-card">
		<table class="w-full text-sm">
			<thead class="border-b text-left text-xs uppercase text-muted-foreground">
				<tr>
					<th class="px-4 py-3">{$_('positions.asset')}</th>
					<th class="px-4 py-3 text-right">{$_('positions.quantity')}</th>
					<th class="px-4 py-3 text-right">{$_('positions.avg_cost')}</th>
					<th class="px-4 py-3 text-right">{$_('positions.current_price')}</th>
					<th class="px-4 py-3 text-right">{$_('positions.market_value')}</th>
					<th class="px-4 py-3 text-right">{$_('positions.unrealized')}</th>
					<th class="px-4 py-3 text-right">{$_('positions.change')}</th>
				</tr>
			</thead>
			<tbody>
				{#each filtered as pos (pos.asset)}
					<tr class="border-b last:border-0 hover:bg-accent/40">
						<td class="px-4 py-3 font-medium">
							<a class="hover:underline" href={`/assets/${pos.asset}`}>{pos.asset}</a>
						</td>
						<td class="px-4 py-3 text-right tabular-nums">
							{formatQuantity(pos.held_quantity)}
						</td>
						<td class="px-4 py-3 text-right">
							<DualMoney money={pos.average_cost} />
						</td>
						<td class="px-4 py-3 text-right">
							<DualMoney money={pos.current_price} />
						</td>
						<td class="px-4 py-3 text-right">
							<DualMoney money={pos.market_value} />
						</td>
						<td class="px-4 py-3 text-right">
							<DualMoney money={pos.unrealized_pnl} class={pnlClass(pos.unrealized_pnl.amount)} />
						</td>
						<td class={`px-4 py-3 text-right tabular-nums ${pnlClass(pos.unrealized_pnl.amount)}`}>
							{formatPctBP(pos.unrealized_pct_bp)}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}
