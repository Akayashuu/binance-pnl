<script lang="ts">
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { api } from '$lib/api/client';
	import type { Trade } from '$lib/api/types';
	import { Input, Label } from '$lib/components/ui';
	import { formatMoney, formatQuantity } from '$lib/utils/format';

	let trades = $state<Trade[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let filter = $state('');

	async function load() {
		loading = true;
		error = null;
		try {
			trades = await api.listTrades(filter || undefined);
		} catch (err) {
			error = (err as Error).message;
		} finally {
			loading = false;
		}
	}

	onMount(load);

	function onFilterChange() {
		void load();
	}
</script>

<div class="flex items-center justify-between">
	<h1 class="text-2xl font-semibold">{$_('trades.title')}</h1>
	<div class="flex items-center gap-2">
		<Label for="filter">{$_('trades.filter')}</Label>
		<Input
			id="filter"
			class="w-32"
			placeholder="BTC"
			bind:value={filter}
			onblur={onFilterChange}
			onkeydown={(e) => e.key === 'Enter' && onFilterChange()}
		/>
	</div>
</div>

{#if loading}
	<p class="mt-6 text-sm text-muted-foreground">{$_('common.loading')}</p>
{:else if error}
	<p class="mt-6 text-sm text-destructive">{error}</p>
{:else if trades.length === 0}
	<p class="mt-6 text-sm text-muted-foreground">{$_('trades.empty')}</p>
{:else}
	<div class="mt-6 overflow-hidden rounded-lg border bg-card">
		<table class="w-full text-sm">
			<thead class="border-b text-left text-xs uppercase text-muted-foreground">
				<tr>
					<th class="px-4 py-3">{$_('trades.date')}</th>
					<th class="px-4 py-3">{$_('trades.asset')}</th>
					<th class="px-4 py-3">{$_('trades.side')}</th>
					<th class="px-4 py-3 text-right">{$_('trades.quantity')}</th>
					<th class="px-4 py-3 text-right">{$_('trades.price')}</th>
					<th class="px-4 py-3 text-right">{$_('trades.fee')}</th>
					<th class="px-4 py-3 text-right">{$_('trades.total')}</th>
				</tr>
			</thead>
			<tbody>
				{#each trades as t (t.id)}
					<tr class="border-b last:border-0 hover:bg-accent/40">
						<td class="px-4 py-3 text-muted-foreground">
							{new Date(t.executed_at).toLocaleString()}
						</td>
						<td class="px-4 py-3 font-medium">
							<a href={`/assets/${t.asset}`} class="hover:underline">{t.asset}</a>
						</td>
						<td class="px-4 py-3">
							<span
								class={`rounded px-2 py-0.5 text-xs font-semibold ${t.side === 'BUY' ? 'bg-success/20 text-success' : 'bg-destructive/20 text-destructive'}`}
							>
								{t.side}
							</span>
						</td>
						<td class="px-4 py-3 text-right tabular-nums">{formatQuantity(t.quantity)}</td>
						<td class="px-4 py-3 text-right tabular-nums">
							{formatMoney(t.price.amount, t.price.currency)}
						</td>
						<td class="px-4 py-3 text-right tabular-nums text-muted-foreground">
							{formatMoney(t.fee.amount, t.fee.currency)}
						</td>
						<td class="px-4 py-3 text-right tabular-nums">
							{formatMoney(t.gross_value.amount, t.gross_value.currency)}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}
