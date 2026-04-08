<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { Button, Card, CardContent, CardHeader, CardTitle, DualMoney } from '$lib/components/ui';
	import AddFundModal from '$lib/components/add-fund-modal.svelte';
	import { portfolio } from '$lib/stores/portfolio';
	import { api } from '$lib/api/client';
	import { formatPctBP, formatQuantity, pnlClass } from '$lib/utils/format';
	import { Plus, RefreshCw } from 'lucide-svelte';

	let syncing = $state(false);
	let syncMessage = $state<string | null>(null);
	let syncErrors = $state<string[]>([]);
	let addFundOpen = $state(false);

	onMount(() => {
		portfolio.startPolling();
	});
	onDestroy(() => {
		portfolio.stopPolling();
	});

	async function handleSync() {
		syncing = true;
		syncMessage = null;
		syncErrors = [];
		try {
			const res = await api.sync();
			// Build a per-source breakdown so the user knows what came from where.
			const parts = Object.entries(res.by_source ?? {})
				.filter(([, count]) => count > 0)
				.map(([source, count]) => `${count} ${source}`);
			const summary =
				parts.length > 0
					? `${$_('dashboard.sync_done', { values: { count: res.imported } })} (${parts.join(', ')})`
					: $_('dashboard.sync_done', { values: { count: res.imported } });
			syncMessage = summary;
			if (res.errors && res.errors.length > 0) {
				syncErrors = res.errors;
			}
			await portfolio.load();
		} catch (err) {
			syncMessage = (err as Error).message;
		} finally {
			syncing = false;
		}
	}
</script>

<div class="flex items-center justify-between">
	<h1 class="text-2xl font-semibold">{$_('nav.dashboard')}</h1>
	<div class="flex gap-2">
		<Button onclick={() => (addFundOpen = true)}>
			<Plus class="mr-2 h-4 w-4" />
			{$_('add_fund.button')}
		</Button>
		<Button onclick={handleSync} disabled={syncing}>
			<RefreshCw class={`mr-2 h-4 w-4 ${syncing ? 'animate-spin' : ''}`} />
			{syncing ? $_('dashboard.syncing') : $_('dashboard.sync')}
		</Button>
	</div>
</div>

<AddFundModal
	open={addFundOpen}
	defaultQuote={$portfolio.data?.quote ?? 'USDT'}
	onClose={() => (addFundOpen = false)}
	onCreated={() => portfolio.load()}
/>

{#if syncMessage}
	<p class="mt-2 text-sm text-muted-foreground">{syncMessage}</p>
{/if}
{#if syncErrors.length > 0}
	<ul class="mt-1 space-y-0.5 text-xs text-destructive">
		{#each syncErrors as e}
			<li>· {e}</li>
		{/each}
	</ul>
{/if}

{#if $portfolio.error}
	<p class="mt-6 text-sm text-destructive">{$portfolio.error}</p>
{:else if !$portfolio.data && $portfolio.loading}
	<p class="mt-6 text-sm text-muted-foreground">{$_('common.loading')}</p>
{:else if $portfolio.data}
	{@const p = $portfolio.data}
	<div class="mt-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
		<Card>
			<CardHeader>
				<CardTitle>{$_('dashboard.total_invested')}</CardTitle>
			</CardHeader>
			<CardContent class="text-2xl font-semibold">
				<DualMoney money={p.total_invested} />
			</CardContent>
		</Card>
		<Card>
			<CardHeader>
				<CardTitle>{$_('dashboard.total_value')}</CardTitle>
			</CardHeader>
			<CardContent class="text-2xl font-semibold">
				<DualMoney money={p.total_value} />
			</CardContent>
		</Card>
		<Card>
			<CardHeader>
				<CardTitle>{$_('dashboard.unrealized_pnl')}</CardTitle>
			</CardHeader>
			<CardContent class={`text-2xl font-semibold ${pnlClass(p.unrealized_pnl.amount)}`}>
				<DualMoney money={p.unrealized_pnl} />
			</CardContent>
		</Card>
		<Card>
			<CardHeader>
				<CardTitle>{$_('dashboard.realized_pnl')}</CardTitle>
			</CardHeader>
			<CardContent class={`text-2xl font-semibold ${pnlClass(p.realized_pnl.amount)}`}>
				<DualMoney money={p.realized_pnl} />
			</CardContent>
		</Card>
	</div>

	<h2 class="mt-10 text-lg font-semibold">{$_('dashboard.positions')}</h2>
	{#if p.positions.length === 0}
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
					{#each p.positions as pos (pos.asset)}
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
							<td
								class={`px-4 py-3 text-right tabular-nums ${pnlClass(pos.unrealized_pnl.amount)}`}
							>
								{formatPctBP(pos.unrealized_pct_bp)}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
{/if}
