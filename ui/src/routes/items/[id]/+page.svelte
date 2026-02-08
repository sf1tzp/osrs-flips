<script lang="ts">
	import { goto } from '$app/navigation';
	import { navigating } from '$app/state';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { ChartContainer, type ChartConfig } from '$lib/components/ui/chart';
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';
	import {
		Area,
		Axis,
		Chart,
		Grid,
		Highlight,
		Spline,
		Svg,
		Tooltip,
	} from 'layerchart';
	import { scaleTime, scaleLinear } from 'd3-scale';
	import {
		SOURCE_OPTIONS,
		SOURCE_LABELS,
		DEFAULT_SOURCES,
		type PriceHistoryRange,
		type PriceHistorySource,
	} from '$lib/price-history';

	const priceTimeScale = scaleTime();
	const priceYScale = scaleLinear();
	const volumeTimeScale = scaleTime();
	const volumeYScale = scaleLinear();
	const pricePadding = { top: 20, right: 60, bottom: 30, left: 10 };
	const priceTooltip = { mode: 'bisect-x' as const };
	const volumePadding = { top: 5, right: 60, bottom: 25, left: 10 };
	const highlightPoints = { class: 'fill-primary r-2' };
	const highlightLines = { class: 'stroke-border' };

	interface ChartPoint {
		time: Date;
		highPrice: number | null;
		lowPrice: number | null;
		highVolume: number | null;
		lowVolume: number | null;
	}

	let { data } = $props();
	let item = $derived(data.item);
	let priceHistory = $derived(data.priceHistory);
	let range = $derived(data.range as PriceHistoryRange);
	let source = $derived(data.source as PriceHistorySource);

	const ranges: { value: PriceHistoryRange; label: string }[] = [
		{ value: '1h', label: '1H' },
		{ value: '6h', label: '6H' },
		{ value: '24h', label: '24H' },
		{ value: '7d', label: '7D' },
		{ value: '30d', label: '30D' },
	];

	let sourceOptions = $derived(SOURCE_OPTIONS[range]);
	let hasVolume = $derived(source !== 'observations');

	function navUrl(r: PriceHistoryRange, s?: PriceHistorySource) {
		const effectiveSource = s ?? DEFAULT_SOURCES[r];
		const params = new URLSearchParams({ range: r });
		if (effectiveSource !== DEFAULT_SOURCES[r]) {
			params.set('source', effectiveSource);
		}
		return `?${params}`;
	}
	let isLoading = $derived(!!navigating.to);

	let chartData = $derived(
		priceHistory.map((p) => ({
			...p,
			time: typeof p.time === 'string' ? new Date(p.time) : p.time,
		}))
	);

	let priceData = $derived(
		chartData.filter((d) => d.highPrice != null || d.lowPrice != null)
	);

	const priceConfig: ChartConfig = {
		highPrice: { label: 'Insta-Buy', color: 'oklch(0.65 0.19 145)' },
		lowPrice: { label: 'Insta-Sell', color: 'oklch(0.65 0.19 25)' },
		margin: { label: 'Margin', color: 'oklch(0.75 0.1 220 / 0.15)' },
	};

	const volumeConfig: ChartConfig = {
		highVolume: { label: 'Buy Volume', color: 'oklch(0.65 0.19 145 / 0.4)' },
		lowVolume: { label: 'Sell Volume', color: 'oklch(0.65 0.19 25 / 0.4)' },
	};

	function formatGp(n: number | null): string {
		if (n == null) return '—';
		return n.toLocaleString() + ' gp';
	}

	function formatVolume(n: number | null): string {
		if (n == null) return '—';
		if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M';
		if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K';
		return n.toLocaleString();
	}

	function formatTime(date: Date): string {
		if (range === '30d') {
			return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
		}
		if (range === '7d') {
			return date.toLocaleDateString(undefined, { weekday: 'short', hour: 'numeric' });
		}
		return date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
	}

	function axisFormat(date: Date): string {
		return formatTime(date);
	}

	function defined(d: { highPrice: number | null; lowPrice: number | null }): boolean {
		return d.highPrice != null && d.lowPrice != null;
	}
</script>

<div
	class="mx-auto max-w-4xl p-4 sm:p-6 transition-opacity duration-150"
	class:opacity-50={isLoading}
>
	<div class="mb-6">
		<Button variant="ghost" href="/" class="mb-4 gap-1.5 px-2">
			<ArrowLeft class="size-4" />
			Back to dashboard
		</Button>
		<div class="flex items-center gap-3">
			{#if item.icon}
				<img src={item.icon} alt={item.name} class="size-10 object-contain" />
			{/if}
			<h1 class="text-2xl font-bold">{item.name}</h1>
			{#if item.members}
				<Badge variant="outline">P2P</Badge>
			{/if}
		</div>
		{#if item.examine}
			<p class="text-muted-foreground mt-1">{item.examine}</p>
		{/if}
	</div>

	<!-- Range & Source Selector -->
	<div class="mb-4 flex items-center gap-1">
		{#each ranges as r (r.value)}
			<Button
				variant={range === r.value ? 'default' : 'ghost'}
				size="sm"
				onclick={() => goto(navUrl(r.value), { keepFocus: true, noScroll: true })}
			>
				{r.label}
			</Button>
		{/each}
		<span class="bg-border mx-2 h-5 w-px"></span>
		{#each sourceOptions as s, i (s)}
			<Button
				variant={source === s ? 'secondary' : 'ghost'}
				size="sm"
				class="text-xs"
				onclick={() => goto(navUrl(range, s), { keepFocus: true, noScroll: true })}
			>
				{SOURCE_LABELS[s]}
			</Button>
		{/each}
	</div>

	{#if priceData.length === 0}
		<div class="text-muted-foreground rounded-lg border p-8 text-center">
			No price data available for this time range.
		</div>
	{:else}
		<!-- Price Chart -->
		<ChartContainer config={priceConfig} class="h-[350px] w-full">
			<Chart
				data={priceData}
				x="time"
				xScale={priceTimeScale}
				yScale={priceYScale}
				y={(d) => d.highPrice ?? d.lowPrice}
				yNice
				padding={pricePadding}
				tooltip={priceTooltip}
			>
				<Svg>
					<Grid y />
					<Area
						y0={(d: ChartPoint) => d.lowPrice}
						y1={(d: ChartPoint) => d.highPrice}
						{defined}
						class="fill-[var(--color-margin)]"
					/>
					<Spline
						y={(d: ChartPoint) => d.highPrice}
						defined={(d: ChartPoint) => d.highPrice != null}
						class="stroke-[var(--color-highPrice)] stroke-[1.5]"
					/>
					<Spline
						y={(d: ChartPoint) => d.lowPrice}
						defined={(d: ChartPoint) => d.lowPrice != null}
						class="stroke-[var(--color-lowPrice)] stroke-[1.5]"
					/>
					<Axis placement="bottom" format={axisFormat} tickSpacing={100} />
					<Axis placement="right" format={(v) => v.toLocaleString()} />
					<Highlight
						points={highlightPoints}
						lines={highlightLines}
					/>
				</Svg>
				<Tooltip.Root
					x="data"
					y="data"
					anchor="top-right"
					variant="none"
					contained="window"
				>
					{#snippet children({ data: d })}
						{#if d}
							<div
								class="border-border/50 bg-background rounded-lg border px-3 py-2 text-xs shadow-xl"
							>
								<div class="mb-1 font-medium">
									{formatTime(d.time)}
								</div>
								<div class="grid gap-1">
									<div class="flex items-center justify-between gap-4">
										<span class="flex items-center gap-1.5">
											<span
												class="size-2.5 rounded-sm bg-[var(--color-highPrice)]"
											></span>
											<span class="text-muted-foreground">Insta-Buy</span>
										</span>
										<span class="font-mono font-medium tabular-nums">
											{formatGp(d.highPrice)}
										</span>
									</div>
									<div class="flex items-center justify-between gap-4">
										<span class="flex items-center gap-1.5">
											<span
												class="size-2.5 rounded-sm bg-[var(--color-lowPrice)]"
											></span>
											<span class="text-muted-foreground">Insta-Sell</span>
										</span>
										<span class="font-mono font-medium tabular-nums">
											{formatGp(d.lowPrice)}
										</span>
									</div>
									{#if d.highPrice != null && d.lowPrice != null}
										{@const tax = Math.min(Math.floor(d.highPrice * 0.02), 5_000_000)}
										{@const margin = d.highPrice - d.lowPrice - tax}
										<div
											class="border-border mt-1 flex items-center justify-between gap-4 border-t pt-1"
										>
											<span class="text-muted-foreground">Margin</span>
											<span class="font-mono font-medium tabular-nums">
												{margin.toLocaleString()} gp
											</span>
										</div>
									{/if}
									{#if d.highVolume != null || d.lowVolume != null}
										<div
											class="border-border flex items-center justify-between gap-4 border-t pt-1"
										>
											<span class="text-muted-foreground">Volume</span>
											<span class="font-mono font-medium tabular-nums">
												{formatVolume(
													(d.highVolume ?? 0) + (d.lowVolume ?? 0)
												)}
											</span>
										</div>
									{/if}
								</div>
							</div>
						{/if}
					{/snippet}
				</Tooltip.Root>
			</Chart>
		</ChartContainer>

		<!-- Volume Chart -->
		{#if hasVolume}
			<ChartContainer config={volumeConfig} class="mt-2 h-[100px] w-full">
				<Chart
					data={chartData}
					x="time"
					xScale={volumeTimeScale}
					yScale={volumeYScale}
					y={(d) => d.highVolume ?? d.lowVolume ?? 0}
					yNice
					padding={volumePadding}
				>
					<Svg>
						<Area
							y="highVolume"
							defined={(d: ChartPoint) => d.highVolume != null}
							class="fill-[var(--color-highVolume)]"
						/>
						<Area
							y="lowVolume"
							defined={(d: ChartPoint) => d.lowVolume != null}
							class="fill-[var(--color-lowVolume)]"
						/>
						<Axis placement="bottom" format={axisFormat} tickSpacing={100} />
					</Svg>
				</Chart>
			</ChartContainer>
		{/if}
	{/if}
</div>
