<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { page } from '$app/stores';
	import { api } from '$lib/api/client';
	import type { AssetDetail, KlinePoint, Trade } from '$lib/api/types';
	import { Card, CardContent, CardHeader, CardTitle, CandlestickChart, DualMoney, HelpHint } from '$lib/components/ui';
	import AddFundModal from '$lib/components/add-fund-modal.svelte';
	import SourceBadge from '$lib/components/source-badge.svelte';
	import { displayCurrency } from '$lib/stores/display-currency';
	import { formatPctBP, formatQuantity, pnlClass } from '$lib/utils/format';
	import { ArrowLeft, Pencil, Trash2 } from 'lucide-svelte';

	let detail = $state<AssetDetail | null>(null);
	let klines = $state<KlinePoint[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let editOpen = $state(false);
	let editing = $state<{
		id: string;
		asset: string;
		quote: string;
		quantity: string;
		unit_cost: string;
		acquired_at: string;
	} | null>(null);

	$effect(() => {
		const symbol = $page.params.symbol;
		const _dc = $displayCurrency; // track currency changes
		void _dc;
		if (symbol) void load(symbol);
	});

	async function load(symbol: string) {
		loading = true;
		error = null;
		try {
			const [d, k] = await Promise.all([
				api.getAssetDetail(symbol),
				api.getKlines(symbol, '1d', 90).catch(() => [] as KlinePoint[])
			]);
			detail = d;
			klines = k;
		} catch (err) {
			error = (err as Error).message;
		} finally {
			loading = false;
		}
	}

	function startEdit(t: Trade) {
		editing = {
			id: t.id,
			asset: t.asset,
			quote: t.price.currency,
			quantity: t.quantity,
			unit_cost: t.price.amount,
			acquired_at: t.executed_at
		};
		editOpen = true;
	}

	async function deleteRow(t: Trade) {
		if (!confirm($_('add_fund.delete_confirm'))) return;
		const sym = $page.params.symbol;
		if (!sym) return;
		try {
			await api.deleteAcquisition(t.id);
			await load(sym);
		} catch (err) {
			error = (err as Error).message;
		}
	}

	function pctClass(pct: string): string {
		const n = Number(pct);
		if (Number.isNaN(n) || n === 0) return 'text-muted-foreground';
		return n > 0 ? 'text-success' : 'text-destructive';
	}

	function isManual(id: string): boolean {
		return id.startsWith('manual-');
	}
</script>

<AddFundModal
	open={editOpen}
	defaultQuote={detail?.pnl.quote ?? 'USDT'}
	editing={editing ?? undefined}
	onClose={() => { editOpen = false; editing = null; }}
	onCreated={() => { const sym = $page.params.symbol; if (sym) void load(sym); }}
/>

<a href="/" class="mb-4 inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground">
	<ArrowLeft class="h-4 w-4" />
	{$_('asset.back')}
</a>

{#if loading}
	<p class="mt-6 text-sm text-muted-foreground">{$_('common.loading')}</p>
{:else if error}
	<p class="mt-6 text-sm text-destructive">{error}</p>
{:else if detail}
	{@const d = detail}
	<h1 class="text-3xl font-semibold">{d.pnl.asset}</h1>

	<!-- Summary cards -->
	<div class="mt-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
		<Card>
			<CardHeader><CardTitle class="flex items-center gap-1.5">{$_('positions.quantity')} <HelpHint text={$_('positions.quantity_help')} /></CardTitle></CardHeader>
			<CardContent class="text-2xl font-semibold tabular-nums">{formatQuantity(d.pnl.held_quantity)}</CardContent>
		</Card>
		<Card>
			<CardHeader><CardTitle class="flex items-center gap-1.5">{$_('positions.avg_cost')} <HelpHint text={$_('positions.avg_cost_help')} /></CardTitle></CardHeader>
			<CardContent class="text-2xl font-semibold"><DualMoney money={d.pnl.average_cost} /></CardContent>
		</Card>
		<Card>
			<CardHeader><CardTitle class="flex items-center gap-1.5">{$_('positions.current_price')} <HelpHint text={$_('positions.current_price_help')} /></CardTitle></CardHeader>
			<CardContent class="text-2xl font-semibold"><DualMoney money={d.pnl.current_price} /></CardContent>
		</Card>
		<Card>
			<CardHeader><CardTitle class="flex items-center gap-1.5">{$_('positions.unrealized')} <HelpHint text={$_('positions.unrealized_help')} /></CardTitle></CardHeader>
			<CardContent class={`text-2xl font-semibold ${pnlClass(d.pnl.unrealized_pnl.amount)}`}>
				<DualMoney money={d.pnl.unrealized_pnl} />
				<div class="mt-1 text-sm">{formatPctBP(d.pnl.unrealized_pct_bp)}</div>
			</CardContent>
		</Card>
	</div>

	<!-- Price chart -->
	{#if klines.length > 1}
		{@const priceAmt = Number(d.pnl.current_price.amount)}
		{@const displayAmt = d.pnl.current_price.display ? Number(d.pnl.current_price.display.amount) : 0}
		{@const rate = displayAmt > 0 && priceAmt > 0 ? displayAmt / priceAmt : 1}
		{@const cur = d.pnl.current_price.display?.currency ?? d.pnl.current_price.currency}
		<div class="mt-6 rounded-lg border bg-card p-4">
			<h2 class="mb-2 text-sm font-semibold text-muted-foreground">{$_('chart.price_90d')} ({cur})</h2>
			<CandlestickChart {klines} {rate} height={320} />
		</div>
	{/if}

	<!-- Lots table -->
	<h2 class="mt-10 text-lg font-semibold">{$_('asset.lots')}</h2>
	{#if d.lots.length === 0}
		<p class="mt-4 text-sm text-muted-foreground">{$_('asset.lots_empty')}</p>
	{:else}
		<div class="mt-4 overflow-hidden rounded-lg border bg-card">
			<table class="w-full text-sm">
				<thead class="border-b text-left text-xs uppercase text-muted-foreground">
					<tr>
						<th class="px-4 py-3">{$_('asset.lot_acquired')}</th>
						<th class="px-4 py-3">{$_('asset.lot_source')}</th>
						<th class="px-4 py-3 text-right">{$_('asset.lot_qty_remaining')}</th>
						<th class="px-4 py-3 text-right">{$_('asset.lot_unit_cost')}</th>
						<th class="px-4 py-3 text-right">{$_('asset.lot_now')}</th>
						<th class="px-4 py-3 text-right">{$_('asset.lot_cost')}</th>
						<th class="px-4 py-3 text-right">{$_('asset.lot_pnl')}</th>
					</tr>
				</thead>
				<tbody>
					{#each d.lots as l (l.acquisition_id)}
						<tr class="border-b last:border-0">
							<td class="px-4 py-3 text-muted-foreground">{new Date(l.acquired_at).toLocaleDateString()}</td>
							<td class="px-4 py-3"><SourceBadge source={l.source} /></td>
							<td class="px-4 py-3 text-right tabular-nums">{formatQuantity(l.remaining_quantity)}</td>
							<td class="px-4 py-3 text-right"><DualMoney money={l.unit_cost} /></td>
							<td class="px-4 py-3 text-right"><DualMoney money={l.current_price} /></td>
							<td class="px-4 py-3 text-right"><DualMoney money={l.cost_basis} /></td>
							<td class="px-4 py-3 text-right">
								<DualMoney money={l.unrealized_pnl} class={pnlClass(l.unrealized_pnl.amount)} />
								<div class={`text-xs ${pctClass(l.unrealized_pnl_pct)}`}>
									{Number(l.unrealized_pnl_pct) > 0 ? '+' : ''}{l.unrealized_pnl_pct}%
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
				<tfoot class="border-t bg-muted/30 text-sm font-semibold">
					<tr>
						<td class="px-4 py-3" colspan={2}>
							<div class="flex items-center gap-1.5">{$_('asset.lots_total')} <HelpHint text={$_('asset.lots_total_help')} /></div>
						</td>
						<td class="px-4 py-3 text-right tabular-nums">{formatQuantity(d.pnl.held_quantity)}</td>
						<td class="px-4 py-3"></td>
						<td class="px-4 py-3 text-right"><DualMoney money={d.pnl.market_value} /></td>
						<td class="px-4 py-3 text-right"><DualMoney money={d.pnl.cost_basis} /></td>
						<td class="px-4 py-3 text-right">
							<DualMoney money={d.pnl.unrealized_pnl} class={pnlClass(d.pnl.unrealized_pnl.amount)} />
							<div class={`text-xs ${pctClass(String(d.pnl.unrealized_pct_bp / 100))}`}>{formatPctBP(d.pnl.unrealized_pct_bp)}</div>
						</td>
					</tr>
				</tfoot>
			</table>
		</div>
	{/if}

	<!-- Trade history -->
	<h2 class="mt-10 text-lg font-semibold">{$_('asset.history')}</h2>
	{#if d.trades.length === 0}
		<p class="mt-4 text-sm text-muted-foreground">{$_('trades.empty')}</p>
	{:else}
		<div class="mt-4 overflow-hidden rounded-lg border bg-card">
			<table class="w-full text-sm">
				<thead class="border-b text-left text-xs uppercase text-muted-foreground">
					<tr>
						<th class="px-4 py-3">{$_('trades.date')}</th>
						<th class="px-4 py-3">{$_('trades.side')}</th>
						<th class="px-4 py-3">{$_('asset.lot_source')}</th>
						<th class="px-4 py-3 text-right">{$_('trades.quantity')}</th>
						<th class="px-4 py-3 text-right">{$_('trades.price')}</th>
						<th class="px-4 py-3 text-right">{$_('trades.total')}</th>
						<th class="px-4 py-3 text-right">{$_('asset.lot_pnl')}</th>
						<th class="px-4 py-3"></th>
					</tr>
				</thead>
				<tbody>
					{#each d.trades as t (t.id)}
						{@const isBuy = t.side === 'BUY'}
						{@const hasDelta = isBuy && Number(t.delta_total.amount) !== 0}
						{@const editable = isManual(t.id)}
						<tr class="border-b last:border-0">
							<td class="px-4 py-3 text-muted-foreground">{new Date(t.executed_at).toLocaleString()}</td>
							<td class="px-4 py-3">
								<span class={`rounded px-2 py-0.5 text-xs font-semibold ${isBuy ? 'bg-success/20 text-success' : 'bg-destructive/20 text-destructive'}`}>
									{t.side}
								</span>
							</td>
							<td class="px-4 py-3"><SourceBadge source={t.source} /></td>
							<td class="px-4 py-3 text-right tabular-nums">{formatQuantity(t.quantity)}</td>
							<td class="px-4 py-3 text-right"><DualMoney money={t.price} /></td>
							<td class="px-4 py-3 text-right"><DualMoney money={t.gross_value} /></td>
							<td class="px-4 py-3 text-right">
								{#if hasDelta}
									<DualMoney money={t.delta_total} class={pnlClass(t.delta_total.amount)} />
									<div class={`text-xs ${pctClass(t.delta_pct)}`}>{Number(t.delta_pct) > 0 ? '+' : ''}{t.delta_pct}%</div>
									{#if Number(t.remaining_qty) > 0 && Number(t.remaining_qty) !== Number(t.quantity)}
										<div class="text-[10px] text-muted-foreground">{formatQuantity(t.remaining_qty)} restant</div>
									{/if}
								{:else}
									<span class="text-xs text-muted-foreground">—</span>
								{/if}
							</td>
							<td class="px-4 py-3 text-right">
								{#if editable}
									<div class="flex justify-end gap-1">
										<button type="button" class="rounded p-1 text-muted-foreground hover:bg-accent hover:text-foreground" onclick={() => startEdit(t)} aria-label="Edit">
											<Pencil class="h-3.5 w-3.5" />
										</button>
										<button type="button" class="rounded p-1 text-muted-foreground hover:bg-destructive/20 hover:text-destructive" onclick={() => deleteRow(t)} aria-label="Delete">
											<Trash2 class="h-3.5 w-3.5" />
										</button>
									</div>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
{/if}
