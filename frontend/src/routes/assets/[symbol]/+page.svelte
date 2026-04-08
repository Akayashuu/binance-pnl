<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { page } from '$app/stores';
	import { api } from '$lib/api/client';
	import type { AssetDetail, Source, Trade } from '$lib/api/types';
	import { Card, CardContent, CardHeader, CardTitle, DualMoney } from '$lib/components/ui';
	import AddFundModal from '$lib/components/add-fund-modal.svelte';
	import { displayCurrency } from '$lib/stores/display-currency';
	import { formatPctBP, formatQuantity, pnlClass } from '$lib/utils/format';
	import { ArrowLeft, Pencil, Trash2 } from 'lucide-svelte';

	const SOURCE_BADGE: Record<Source, string> = {
		spot: 'bg-blue-500/20 text-blue-300',
		convert: 'bg-purple-500/20 text-purple-300',
		fiat_buy: 'bg-emerald-500/20 text-emerald-300',
		recurring: 'bg-cyan-500/20 text-cyan-300',
		deposit: 'bg-amber-500/20 text-amber-300',
		earn_reward: 'bg-pink-500/20 text-pink-300'
	};

	function pctClass(pct: string): string {
		const n = Number(pct);
		if (Number.isNaN(n) || n === 0) return 'text-muted-foreground';
		return n > 0 ? 'text-success' : 'text-destructive';
	}

	function isManual(id: string): boolean {
		return id.startsWith('manual-');
	}

	let detail = $state<AssetDetail | null>(null);
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
		$displayCurrency;
		if (symbol) void load(symbol);
	});

	async function load(symbol: string) {
		loading = true;
		error = null;
		try {
			detail = await api.getAssetDetail(symbol);
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
</script>

<AddFundModal
	open={editOpen}
	defaultQuote={detail?.pnl.quote ?? 'USDT'}
	editing={editing ?? undefined}
	onClose={() => {
		editOpen = false;
		editing = null;
	}}
	onCreated={() => {
		const sym = $page.params.symbol;
		if (sym) void load(sym);
	}}
/>

<a
	href="/"
	class="mb-4 inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground"
>
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

	<div class="mt-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
		<Card>
			<CardHeader>
				<CardTitle>{$_('positions.quantity')}</CardTitle>
			</CardHeader>
			<CardContent class="text-2xl font-semibold tabular-nums">
				{formatQuantity(d.pnl.held_quantity)}
			</CardContent>
		</Card>
		<Card>
			<CardHeader>
				<CardTitle>{$_('positions.avg_cost')}</CardTitle>
			</CardHeader>
			<CardContent class="text-2xl font-semibold">
				<DualMoney money={d.pnl.average_cost} />
			</CardContent>
		</Card>
		<Card>
			<CardHeader>
				<CardTitle>{$_('positions.current_price')}</CardTitle>
			</CardHeader>
			<CardContent class="text-2xl font-semibold">
				<DualMoney money={d.pnl.current_price} />
			</CardContent>
		</Card>
		<Card>
			<CardHeader>
				<CardTitle>{$_('positions.unrealized')}</CardTitle>
			</CardHeader>
			<CardContent class={`text-2xl font-semibold ${pnlClass(d.pnl.unrealized_pnl.amount)}`}>
				<DualMoney money={d.pnl.unrealized_pnl} />
				<div class="mt-1 text-sm">{formatPctBP(d.pnl.unrealized_pct_bp)}</div>
			</CardContent>
		</Card>
	</div>

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
							<td class="px-4 py-3 text-muted-foreground">
								{new Date(l.acquired_at).toLocaleDateString()}
							</td>
							<td class="px-4 py-3">
								<span
									class={`rounded px-2 py-0.5 text-xs font-semibold ${SOURCE_BADGE[l.source] ?? 'bg-muted text-muted-foreground'}`}
								>
									{$_(`sources.${l.source}`)}
								</span>
							</td>
							<td class="px-4 py-3 text-right tabular-nums">
								{formatQuantity(l.remaining_quantity)}
							</td>
							<td class="px-4 py-3 text-right">
								<DualMoney money={l.unit_cost} />
							</td>
							<td class="px-4 py-3 text-right">
								<DualMoney money={l.current_price} />
							</td>
							<td class="px-4 py-3 text-right">
								<DualMoney money={l.cost_basis} />
							</td>
							<td class="px-4 py-3 text-right">
								<DualMoney money={l.unrealized_pnl} class={pnlClass(l.unrealized_pnl.amount)} />
								<div class={`text-xs ${pctClass(l.unrealized_pnl_pct)}`}>
									{Number(l.unrealized_pnl_pct) > 0 ? '+' : ''}{l.unrealized_pnl_pct}%
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}

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
							<td class="px-4 py-3 text-muted-foreground">
								{new Date(t.executed_at).toLocaleString()}
							</td>
							<td class="px-4 py-3">
								<span
									class={`rounded px-2 py-0.5 text-xs font-semibold ${isBuy ? 'bg-success/20 text-success' : 'bg-destructive/20 text-destructive'}`}
								>
									{t.side}
								</span>
							</td>
							<td class="px-4 py-3">
								<span
									class={`rounded px-2 py-0.5 text-xs font-semibold ${SOURCE_BADGE[t.source] ?? 'bg-muted text-muted-foreground'}`}
								>
									{$_(`sources.${t.source}`)}
								</span>
							</td>
							<td class="px-4 py-3 text-right tabular-nums">{formatQuantity(t.quantity)}</td>
							<td class="px-4 py-3 text-right">
								<DualMoney money={t.price} />
							</td>
							<td class="px-4 py-3 text-right">
								<DualMoney money={t.gross_value} />
							</td>
							<td class="px-4 py-3 text-right">
								{#if hasDelta}
									<DualMoney money={t.delta_total} class={pnlClass(t.delta_total.amount)} />
									<div class={`text-xs ${pctClass(t.delta_pct)}`}>
										{Number(t.delta_pct) > 0 ? '+' : ''}{t.delta_pct}%
									</div>
									{#if Number(t.remaining_qty) > 0 && Number(t.remaining_qty) !== Number(t.quantity)}
										<div class="text-[10px] text-muted-foreground">
											{formatQuantity(t.remaining_qty)} restant
										</div>
									{/if}
								{:else}
									<span class="text-xs text-muted-foreground">—</span>
								{/if}
							</td>
							<td class="px-4 py-3 text-right">
								{#if editable}
									<div class="flex justify-end gap-1">
										<button
											type="button"
											class="rounded p-1 text-muted-foreground hover:bg-accent hover:text-foreground"
											onclick={() => startEdit(t)}
											aria-label="Edit"
										>
											<Pencil class="h-3.5 w-3.5" />
										</button>
										<button
											type="button"
											class="rounded p-1 text-muted-foreground hover:bg-destructive/20 hover:text-destructive"
											onclick={() => deleteRow(t)}
											aria-label="Delete"
										>
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
