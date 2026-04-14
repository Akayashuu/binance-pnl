<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { Button, Card, CardContent, CardHeader, CardTitle, DualMoney, HelpHint } from '$lib/components/ui';
	import PortfolioChart from '$lib/components/portfolio-chart.svelte';
	import PositionsTable from '$lib/components/positions-table.svelte';
	import SyncActions from '$lib/components/sync-actions.svelte';
	import AddFundModal from '$lib/components/add-fund-modal.svelte';
	import { portfolio } from '$lib/stores/portfolio';
	import { pnlClass } from '$lib/utils/format';
	import { Plus } from 'lucide-svelte';

	let addFundOpen = $state(false);

	onMount(() => portfolio.startPolling());
	onDestroy(() => portfolio.stopPolling());
</script>

<div class="flex items-center justify-between">
	<h1 class="text-2xl font-semibold">{$_('nav.dashboard')}</h1>
	<div class="flex gap-2">
		<Button onclick={() => (addFundOpen = true)}>
			<Plus class="mr-2 h-4 w-4" />
			{$_('add_fund.button')}
		</Button>
		<SyncActions />
	</div>
</div>

<AddFundModal
	open={addFundOpen}
	defaultQuote={$portfolio.data?.quote ?? 'USDT'}
	onClose={() => (addFundOpen = false)}
	onCreated={() => portfolio.load()}
/>

{#if $portfolio.error}
	<p class="mt-6 text-sm text-destructive">{$portfolio.error}</p>
{:else if !$portfolio.data && $portfolio.loading}
	<p class="mt-6 text-sm text-muted-foreground">{$_('common.loading')}</p>
{:else if $portfolio.data}
	{@const p = $portfolio.data}

	<div class="mt-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
		<Card>
			<CardHeader>
				<CardTitle class="flex items-center gap-1.5">
					{$_('dashboard.total_invested')}
					<HelpHint text={$_('dashboard.total_invested_help')} />
				</CardTitle>
			</CardHeader>
			<CardContent class="text-2xl font-semibold">
				<DualMoney money={p.total_invested} />
			</CardContent>
		</Card>
		<Card>
			<CardHeader>
				<CardTitle class="flex items-center gap-1.5">
					{$_('dashboard.total_value')}
					<HelpHint text={$_('dashboard.total_value_help')} />
				</CardTitle>
			</CardHeader>
			<CardContent class="text-2xl font-semibold">
				<DualMoney money={p.total_value} />
			</CardContent>
		</Card>
		<Card>
			<CardHeader>
				<CardTitle class="flex items-center gap-1.5">
					{$_('dashboard.unrealized_pnl')}
					<HelpHint text={$_('dashboard.unrealized_pnl_help')} />
				</CardTitle>
			</CardHeader>
			<CardContent class={`text-2xl font-semibold ${pnlClass(p.unrealized_pnl.amount)}`}>
				<DualMoney money={p.unrealized_pnl} />
			</CardContent>
		</Card>
		<Card>
			<CardHeader>
				<CardTitle class="flex items-center gap-1.5">
					{$_('dashboard.realized_pnl')}
					<HelpHint text={$_('dashboard.realized_pnl_help')} />
				</CardTitle>
			</CardHeader>
			<CardContent class={`text-2xl font-semibold ${pnlClass(p.realized_pnl.amount)}`}>
				<DualMoney money={p.realized_pnl} />
			</CardContent>
		</Card>
	</div>

	<PortfolioChart portfolio={p} />

	<h2 class="mt-10 text-lg font-semibold">{$_('dashboard.positions')}</h2>
	<PositionsTable positions={p.positions} />
{/if}
