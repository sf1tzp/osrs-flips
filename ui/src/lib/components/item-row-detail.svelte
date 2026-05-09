<script lang="ts">
	import { Chart, Spline, Svg } from 'layerchart';
	import { scaleTime, scaleLinear } from 'd3-scale';

	interface Props {
		itemId: number;
	}

	let { itemId }: Props = $props();

	interface SparklinePoint {
		time: Date;
		highPrice: number;
	}

	let loading = $state(true);
	let error = $state<string | null>(null);
	let prices = $state<SparklinePoint[]>([]);
	let avgDailyVolume = $state<number | null>(null);

	const xScale = scaleTime();
	const yScale = scaleLinear();

	function formatVolume(n: number | null): string {
		if (n == null) return '—';
		if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M';
		if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K';
		return n.toLocaleString();
	}

	$effect(() => {
		const controller = new AbortController();
		loading = true;
		error = null;

		fetch(`/api/items/${itemId}/sparkline`, { signal: controller.signal })
			.then((res) => {
				if (!res.ok) throw new Error('Failed to load');
				return res.json();
			})
			.then((data) => {
				prices = data.prices.map((p: { time: string; highPrice: number }) => ({
					time: new Date(p.time),
					highPrice: p.highPrice
				}));
				avgDailyVolume = data.avgDailyVolume;
				loading = false;
			})
			.catch((err) => {
				if (err.name !== 'AbortError') {
					error = 'Failed to load sparkline data';
					loading = false;
				}
			});

		return () => controller.abort();
	});
</script>

<div class="flex items-center gap-8 px-4 py-3">
	{#if loading}
		<div class="flex items-center gap-8">
			<div class="h-[80px] w-[200px] animate-pulse rounded bg-muted"></div>
			<div class="space-y-2">
				<div class="h-3 w-24 animate-pulse rounded bg-muted"></div>
				<div class="h-5 w-16 animate-pulse rounded bg-muted"></div>
			</div>
		</div>
	{:else if error}
		<p class="text-sm text-muted-foreground">{error}</p>
	{:else if prices.length > 0}
		<div>
			<p class="mb-1 text-xs text-muted-foreground">7d High Price</p>
			<div class="h-[80px] w-[200px]">
				<Chart
					data={prices}
					x="time"
					y="highPrice"
					{xScale}
					{yScale}
					yNice
					padding={{ top: 4, right: 2, bottom: 4, left: 2 }}
				>
					<Svg>
						<Spline class="fill-none stroke-primary stroke-[1.5]" />
					</Svg>
				</Chart>
			</div>
		</div>
		<div>
			<p class="text-xs text-muted-foreground">Avg Daily Volume</p>
			<p class="text-sm font-medium tabular-nums">{formatVolume(avgDailyVolume)}</p>
		</div>
	{:else}
		<p class="text-sm text-muted-foreground">No price data available</p>
	{/if}
</div>
