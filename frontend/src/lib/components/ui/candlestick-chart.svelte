<script lang="ts">
	import { createChart, CandlestickSeries, HistogramSeries, type IChartApi } from 'lightweight-charts';
	import type { KlinePoint } from '$lib/api/types';

	interface Props {
		klines: KlinePoint[];
		/** FX rate to convert quote → display currency */
		rate?: number;
		height?: number;
	}

	let { klines, rate = 1, height = 320 }: Props = $props();

	function chartAction(node: HTMLDivElement, input: { klines: KlinePoint[]; rate: number }) {
		let chart: IChartApi | undefined;

		function render(inp: { klines: KlinePoint[]; rate: number }) {
			if (chart) {
				chart.remove();
				chart = undefined;
			}
			if (inp.klines.length < 2) return;

			chart = createChart(node, {
				height,
				layout: {
					background: { color: 'transparent' },
					textColor: '#666'
				},
				grid: {
					vertLines: { color: '#ffffff08' },
					horzLines: { color: '#ffffff08' }
				},
				rightPriceScale: {
					borderVisible: false
				},
				timeScale: {
					borderVisible: false,
					timeVisible: false
				},
				crosshair: {
					horzLine: { color: '#ffffff30', labelBackgroundColor: '#333' },
					vertLine: { color: '#ffffff30', labelBackgroundColor: '#333' }
				}
			});

			const r = inp.rate;

			const candleSeries = chart.addSeries(CandlestickSeries, {
				upColor: '#22c55e',
				downColor: '#ef4444',
				borderDownColor: '#ef4444',
				borderUpColor: '#22c55e',
				wickDownColor: '#ef4444',
				wickUpColor: '#22c55e'
			});

			candleSeries.setData(
				inp.klines.map((k) => ({
					time: (k.time / 1000) as any,
					open: Number(k.open) * r,
					high: Number(k.high) * r,
					low: Number(k.low) * r,
					close: Number(k.close) * r
				}))
			);

			const volumeSeries = chart.addSeries(HistogramSeries, {
				priceFormat: { type: 'volume' },
				priceScaleId: 'volume'
			});

			chart.priceScale('volume').applyOptions({
				scaleMargins: { top: 0.8, bottom: 0 }
			});

			volumeSeries.setData(
				inp.klines.map((k) => ({
					time: (k.time / 1000) as any,
					value: Number(k.volume),
					color: Number(k.close) >= Number(k.open) ? '#22c55e40' : '#ef444440'
				}))
			);

			chart.timeScale().fitContent();
		}

		render(input);

		return {
			update(next: { klines: KlinePoint[]; rate: number }) {
				render(next);
			},
			destroy() {
				if (chart) chart.remove();
			}
		};
	}
</script>

<div use:chartAction={{ klines, rate }} style="height: {height}px"></div>
