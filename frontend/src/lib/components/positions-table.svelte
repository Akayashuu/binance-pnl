<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { DualMoney } from '$lib/components/ui';
	import { formatPctBP, formatQuantity, pnlClass } from '$lib/utils/format';
	import type { PnL } from '$lib/api/types';

	interface Props {
		positions: PnL[];
	}

	let { positions }: Props = $props();
</script>

{#if positions.length === 0}
	<p class="mt-4 text-sm text-muted-foreground">{$_('dashboard.empty')}</p>
{:else}
	<div class="mt-4 overflow-hidden rounded-lg border bg-card">
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
				{#each positions as pos (pos.asset)}
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
