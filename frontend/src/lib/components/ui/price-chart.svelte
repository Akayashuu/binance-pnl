<script lang="ts">
	import {
		Chart,
		LineController,
		LineElement,
		PointElement,
		LinearScale,
		CategoryScale,
		Filler,
		Tooltip,
		Legend,
		type TooltipItem
	} from 'chart.js';

	Chart.register(
		LineController,
		LineElement,
		PointElement,
		LinearScale,
		CategoryScale,
		Filler,
		Tooltip,
		Legend
	);

	export interface Dataset {
		label: string;
		data: number[];
		color: string;
	}

	interface Props {
		labels: string[];
		datasets: Dataset[];
		height?: number;
	}

	let { labels, datasets, height = 280 }: Props = $props();

	interface ChartInput {
		labels: string[];
		datasets: Dataset[];
	}

	function chartAction(node: HTMLCanvasElement, input: ChartInput) {
		let instance: Chart | undefined;

		function render(inp: ChartInput) {
			if (instance) instance.destroy();
			if (inp.datasets.length === 0) return;
			const ctx = node.getContext('2d');
			if (!ctx) return;

			const multi = inp.datasets.length > 1;

			instance = new Chart(ctx, {
				type: 'line',
				data: {
					labels: inp.labels,
					datasets: inp.datasets.map((d) => ({
						label: d.label,
						data: d.data,
						borderColor: d.color,
						backgroundColor: multi ? 'transparent' : d.color + '15',
						borderWidth: multi ? 1.5 : 2,
						fill: !multi,
						tension: 0.3,
						pointRadius: 0,
						pointHitRadius: 10
					}))
				},
				options: {
					responsive: true,
					maintainAspectRatio: false,
					animation: false,
					plugins: {
						legend: {
							display: multi,
							position: 'top',
							labels: {
								color: '#999',
								usePointStyle: true,
								pointStyle: 'line',
								boxWidth: 20,
								padding: 16,
								font: { size: 11 }
							}
						},
						tooltip: {
							mode: 'index',
							intersect: false,
							callbacks: {
								label: (item: TooltipItem<'line'>) => {
									if (item.parsed.y == null) return '';
									const val = item.parsed.y.toLocaleString(undefined, {
										minimumFractionDigits: 2,
										maximumFractionDigits: 2
									});
									return multi ? `${item.dataset.label}: ${val}` : val;
								}
							}
						}
					},
					scales: {
						x: {
							display: true,
							ticks: { maxTicksLimit: 6, color: '#666', font: { size: 10 } },
							grid: { display: false }
						},
						y: {
							display: true,
							position: 'right',
							ticks: {
								maxTicksLimit: 5,
								color: '#666',
								font: { size: 10 },
								callback: (v: number | string) =>
									Number(v).toLocaleString(undefined, {
										minimumFractionDigits: 0,
										maximumFractionDigits: 2
									})
							},
							grid: { color: '#ffffff08' }
						}
					},
					interaction: { mode: 'index', intersect: false }
				}
			});
		}

		render(input);
		return {
			update(next: ChartInput) {
				render(next);
			},
			destroy() {
				if (instance) instance.destroy();
			}
		};
	}
</script>

<div style="height: {height}px">
	<canvas use:chartAction={{ labels, datasets }}></canvas>
</div>
