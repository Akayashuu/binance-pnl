<script lang="ts">
	import { _ } from 'svelte-i18n';
	import { PriceChart } from '$lib/components/ui';
	import type { Dataset } from '$lib/components/ui/price-chart.svelte';
	import type { KlinePoint, Portfolio } from '$lib/api/types';
	import { api } from '$lib/api/client';
	import { assetColor } from '$lib/utils/asset-colors';

	interface Props {
		portfolio: Portfolio;
	}

	let { portfolio: p }: Props = $props();

	// --- Period config ---
	interface Period {
		label: string;
		interval: string;
		limit: number;
		labelFmt: Intl.DateTimeFormatOptions;
	}

	const PERIODS: Period[] = [
		{ label: '24h', interval: '15m', limit: 96, labelFmt: { hour: '2-digit', minute: '2-digit' } },
		{ label: '7d', interval: '1h', limit: 168, labelFmt: { month: 'short', day: 'numeric' } },
		{ label: '30d', interval: '4h', limit: 180, labelFmt: { month: 'short', day: 'numeric' } },
		{ label: '90d', interval: '1d', limit: 90, labelFmt: { month: 'short', day: 'numeric' } },
		{ label: '1y', interval: '1d', limit: 365, labelFmt: { month: 'short', year: '2-digit' } }
	];

	type ChartMode = 'portfolio' | 'all' | string;

	let mode: ChartMode = $state('portfolio');
	let activePeriod: Period = $state(PERIODS[3]); // default 90d
	let labels: string[] = $state([]);
	let datasets: Dataset[] = $state([]);
	let chartableAssets: string[] = $state([]);
	let loadingChart = $state(false);

	// Internal caches — keyed by period label so switching period re-fetches.
	let cachedPeriod = '';
	let klines = new Map<string, KlinePoint[]>();
	let qtys = new Map<string, number>();
	let cashValue = 0;
	let fxRate = 1;
	let currency = '';
	let times: number[] = [];

	function deriveFx(port: Portfolio) {
		const quote = Number(port.total_value.amount);
		const display = port.total_value.display ? Number(port.total_value.display.amount) : 0;
		if (display > 0 && quote > 0) {
			fxRate = display / quote;
			currency = port.total_value.display?.currency ?? port.total_value.currency;
		} else {
			fxRate = 1;
			currency = port.total_value.currency;
		}
	}

	function deriveCash(port: Portfolio) {
		cashValue =
			port.positions
				.filter(
					(pos) => Number(pos.held_quantity) > 0 && Number(pos.current_price.amount) <= 1.1
				)
				.reduce((sum, pos) => sum + Number(pos.market_value.amount), 0) * fxRate;
	}

	async function fetchKlines(port: Portfolio, period: Period) {
		deriveFx(port);
		deriveCash(port);

		const assets = port.positions.filter(
			(pos) => Number(pos.held_quantity) > 0 && Number(pos.current_price.amount) > 1.1
		);
		if (assets.length === 0) return;

		chartableAssets = assets.map((a) => a.asset);
		qtys = new Map(assets.map((a) => [a.asset, Number(a.held_quantity)]));

		loadingChart = true;
		const results = await Promise.all(
			assets.map((a) =>
				api.getKlines(a.asset, period.interval, period.limit).catch(() => [] as KlinePoint[])
			)
		);
		loadingChart = false;

		klines = new Map();
		assets.forEach((a, i) => klines.set(a.asset, results[i]));

		times = [];
		for (const k of klines.values()) {
			if (k.length > times.length) times = k.map((x) => x.time);
		}

		cachedPeriod = period.label;
		apply(mode);
	}

	function apply(m: ChartMode) {
		if (times.length < 2) return;

		const timeIdx = new Map(times.map((t, i) => [t, i]));
		const lbls = times.map((t) =>
			new Date(t).toLocaleDateString(undefined, activePeriod.labelFmt)
		);
		const r = fxRate;
		const cur = currency;

		if (m === 'portfolio') {
			const vals = new Array(times.length).fill(cashValue);
			for (const [sym, kl] of klines) {
				const qty = qtys.get(sym) ?? 0;
				if (qty === 0) continue;
				for (const k of kl) {
					const i = timeIdx.get(k.time);
					if (i !== undefined) vals[i] += qty * Number(k.close) * r;
				}
			}
			const up = vals[vals.length - 1] >= vals[0];
			labels = lbls;
			datasets = [
				{
					label: `${$_('chart.portfolio')} (${cur})`,
					data: vals,
					color: up ? '#22c55e' : '#ef4444'
				}
			];
		} else if (m === 'all') {
			labels = lbls;
			datasets = [...klines.entries()].map(([sym, kl]) => {
				const kMap = new Map(kl.map((k) => [k.time, Number(k.close) * r]));
				return {
					label: sym,
					data: times.map((t) => kMap.get(t) ?? NaN),
					color: assetColor(sym)
				};
			});
		} else {
			const kl = klines.get(m);
			if (!kl) return;
			const kMap = new Map(kl.map((k) => [k.time, Number(k.close) * r]));
			const vals = times.map((t) => kMap.get(t) ?? NaN);
			const clean = vals.filter((v) => !isNaN(v));
			const up = clean.length >= 2 && clean[clean.length - 1] >= clean[0];
			labels = lbls;
			datasets = [{ label: `${m} (${cur})`, data: vals, color: up ? '#22c55e' : '#ef4444' }];
		}
	}

	function setMode(m: ChartMode) {
		mode = m;
		apply(m);
	}

	function setPeriod(period: Period) {
		activePeriod = period;
		void fetchKlines(p, period);
	}

	// Load on first render; re-apply FX on portfolio updates (currency switch).
	$effect(() => {
		if (!p || p.positions.length === 0) return;
		if (cachedPeriod !== activePeriod.label) {
			void fetchKlines(p, activePeriod);
		} else {
			deriveFx(p);
			deriveCash(p);
			apply(mode);
		}
	});
</script>

{#if datasets.length > 0 || loadingChart}
	<div class="mt-6 rounded-lg border bg-card p-4">
		<!-- Top row: asset selector + period selector -->
		<div class="mb-3 flex items-center justify-between">
			<div class="flex items-center gap-1.5">
				<button
					class="rounded-full px-3 py-1 text-xs font-medium transition-colors
						{mode === 'portfolio'
						? 'bg-primary text-primary-foreground'
						: 'text-muted-foreground hover:text-foreground'}"
					onclick={() => setMode('portfolio')}
				>
					{$_('chart.portfolio')}
				</button>
				{#if chartableAssets.length > 1}
					<button
						class="rounded-full px-3 py-1 text-xs font-medium transition-colors
							{mode === 'all'
							? 'bg-primary text-primary-foreground'
							: 'text-muted-foreground hover:text-foreground'}"
						onclick={() => setMode('all')}
					>
						{$_('chart.all_assets')}
					</button>
				{/if}
				<span class="mx-1 h-4 w-px bg-border"></span>
				{#each chartableAssets as asset}
					<button
						class="rounded-full px-3 py-1 text-xs font-medium transition-colors"
						style={mode === asset
							? `background:${assetColor(asset)};color:#fff`
							: `color:${assetColor(asset)}`}
						onclick={() => setMode(asset)}
					>
						{asset}
					</button>
				{/each}
			</div>

			<div class="flex items-center gap-0.5 rounded-full border p-0.5">
				{#each PERIODS as period}
					<button
						class="rounded-full px-2.5 py-0.5 text-xs font-medium transition-colors
							{activePeriod.label === period.label
							? 'bg-primary text-primary-foreground'
							: 'text-muted-foreground hover:text-foreground'}"
						onclick={() => setPeriod(period)}
					>
						{period.label}
					</button>
				{/each}
			</div>
		</div>

		{#if loadingChart}
			<div class="flex h-[300px] items-center justify-center text-sm text-muted-foreground">
				{$_('common.loading')}
			</div>
		{:else}
			<PriceChart
				labels={[...labels]}
				datasets={datasets.map((d) => ({ ...d, data: [...d.data] }))}
				height={300}
			/>
		{/if}
	</div>
{/if}
