<script lang="ts">
  import { goto } from '$app/navigation';
  import { navigating, page } from '$app/state';
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { ChartContainer, type ChartConfig } from '$lib/components/ui/chart';
  import ArrowLeft from '@lucide/svelte/icons/arrow-left';
  import ChevronDown from '@lucide/svelte/icons/chevron-down';
  import { Area, Axis, Chart, Grid, Highlight, Spline, Svg, Tooltip } from 'layerchart';
  import { scaleTime, scaleLinear } from 'd3-scale';
  import {
    SOURCE_OPTIONS,
    SOURCE_LABELS,
    DEFAULT_SOURCES,
    type PriceHistoryRange,
    type PriceHistorySource
  } from '$lib/price-history';
  import type { ActiveSignal, BucketCoverage } from '$lib/server/db/queries';
  import TradeDialog from '$lib/components/trade-dialog.svelte';
  import { tradeStore } from '$lib/portfolio/trade-store.svelte';
  import { calcGeTax } from '$lib/portfolio/types';

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
  let coverage = $derived(data.coverage as BucketCoverage[]);
  let range = $derived(data.range as PriceHistoryRange);
  let source = $derived(data.source as PriceHistorySource);

  const BACK_ROUTES: Record<string, { href: string; label: string }> = {
    signals: { href: '/signals', label: 'Back to signals' },
    momentum: { href: '/signals/momentum', label: 'Back to signals' },
    portfolio: { href: '/portfolio', label: 'Back to portfolio' }
  };

  let backLink = $derived.by(() => {
    const from = page.url.searchParams.get('from');
    return (from && BACK_ROUTES[from]) || { href: '/', label: 'Back to dashboard' };
  });

  $effect(() => {
    tradeStore.init();
  });

  let itemTrades = $derived(
    tradeStore.trades.filter(
      (t) => t.itemId === item.itemId && (t.status === 'active' || t.status === 'pending')
    )
  );

  let signals = $derived(data.signals as ActiveSignal[]);

  let sma24h = $derived.by(() => {
    const prices = (data.smaHistory as { lowPrice: number | null }[])
      .map((p) => p.lowPrice)
      .filter((v): v is number => v != null);
    if (prices.length === 0) return null;
    return Math.round(prices.reduce((sum, p) => sum + p, 0) / prices.length);
  });

  function signalLabel(type: string): string {
    return type.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
  }

  function faqAnchor(type: string): string {
    const map: Record<string, string> = {
      spread_widening: 'spread-widening',
      price_inversion: 'price-inversion',
      macd_crossover: 'macd',
      rsi_oversold: 'rsi'
    };
    return `/faq#${map[type] ?? type}`;
  }

  function isMomentumSignal(type: string): boolean {
    return type === 'macd_crossover' || type === 'rsi_oversold';
  }

  let tradeDialogOpen = $state(false);

  // Latest prices from history (last data point)
  let latestHigh = $derived(priceHistory.findLast((p) => p.highPrice != null)?.highPrice ?? 0);
  let latestLow = $derived(priceHistory.findLast((p) => p.lowPrice != null)?.lowPrice ?? 0);

  let tradeDialogPrefill = $derived({
    itemId: item.itemId,
    itemName: item.name,
    itemIcon: item.icon,
    instaBuyPrice: latestHigh,
    instaSellPrice: latestLow
  });

  function openFlip() {
    tradeDialogOpen = true;
  }

  const ranges: { value: PriceHistoryRange; label: string }[] = [
    { value: '1h', label: '1H' },
    { value: '6h', label: '6H' },
    { value: '24h', label: '24H' },
    { value: '7d', label: '7D' },
    { value: '30d', label: '30D' }
  ];

  let sourceOptions = $derived(SOURCE_OPTIONS[range]);
  let hasVolume = $derived(source !== 'observations');

  function navUrl(r: PriceHistoryRange, s?: PriceHistorySource) {
    const effectiveSource = s ?? DEFAULT_SOURCES[r];
    const params = new URLSearchParams({ range: r });
    if (effectiveSource !== DEFAULT_SOURCES[r]) {
      params.set('source', effectiveSource);
    }
    const from = page.url.searchParams.get('from');
    if (from) {
      params.set('from', from);
    }
    return `?${params}`;
  }
  let isLoading = $derived(!!navigating.to);

  let chartData = $derived(
    priceHistory.map((p) => ({
      ...p,
      time: typeof p.time === 'string' ? new Date(p.time) : p.time
    }))
  );

  let priceData = $derived(chartData.filter((d) => d.highPrice != null || d.lowPrice != null));

  let highPriceData = $derived(priceData.filter((d) => d.highPrice != null));
  let lowPriceData = $derived(priceData.filter((d) => d.lowPrice != null));

  let priceYDomain = $derived.by(() => {
    const prices = priceData.flatMap((d) =>
      [d.highPrice, d.lowPrice].filter((v): v is number => v != null)
    );
    if (prices.length === 0) return undefined;
    return [Math.min(...prices), Math.max(...prices)];
  });

  const priceConfig: ChartConfig = {
    highPrice: { label: 'Insta-Buy', color: 'oklch(0.65 0.19 145)' },
    lowPrice: { label: 'Insta-Sell', color: 'oklch(0.65 0.19 25)' },
    margin: { label: 'Margin', color: 'oklch(0.75 0.1 220 / 0.15)' }
  };

  const volumeConfig: ChartConfig = {
    highVolume: { label: 'Buy Volume', color: 'oklch(0.65 0.19 145 / 0.4)' },
    lowVolume: { label: 'Sell Volume', color: 'oklch(0.65 0.19 25 / 0.4)' }
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

  function formatDate(iso: string): string {
    const d = new Date(iso);
    return d.toLocaleDateString(undefined, {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
  }

  function formatDuration(ms: number): string {
    const minutes = Math.round(ms / 60_000);
    if (minutes < 60) return `${minutes}m`;
    const hours = Math.floor(minutes / 60);
    const mins = minutes % 60;
    if (hours < 24) return mins > 0 ? `${hours}h ${mins}m` : `${hours}h`;
    const days = Math.floor(hours / 24);
    const hrs = hours % 24;
    return hrs > 0 ? `${days}d ${hrs}h` : `${days}d`;
  }

  function coveragePct(c: BucketCoverage): number {
    if (c.expectedBuckets === 0) return 0;
    return Math.min((c.totalBuckets / c.expectedBuckets) * 100, 100);
  }

  function dataAge(iso: string): string {
    const ms = Date.now() - new Date(iso).getTime();
    return formatDuration(ms) + ' ago';
  }
</script>

<div
  class="mx-auto max-w-4xl p-4 sm:p-6 transition-opacity duration-150"
  class:opacity-50={isLoading}
>
  <div class="mb-6">
    <Button variant="ghost" href={backLink.href} class="mb-4 gap-1.5 px-2">
      <ArrowLeft class="size-4" />
      {backLink.label}
    </Button>
    <div class="flex items-center gap-3">
      {#if item.icon}
        <img src={item.icon} alt={item.name} class="size-10 object-contain" />
      {/if}
      <h1 class="text-2xl font-bold">{item.name}</h1>
      {#if item.members}
        <Badge variant="outline">P2P</Badge>
      {/if}
      <div class="ml-auto">
        <Button size="sm" variant="outline" onclick={openFlip}>Flip</Button>
      </div>
    </div>
    {#if item.examine}
      <p class="text-muted-foreground mt-1">{item.examine}</p>
    {/if}

    <!-- Latest prices -->
    {#if latestHigh > 0 || latestLow > 0}
      {@const margin =
        latestHigh > 0 && latestLow > 0
          ? latestHigh - latestLow - Math.min(Math.floor(latestHigh * 0.02), 5_000_000)
          : null}
      <div class="mt-3 flex flex-wrap items-center gap-x-6 gap-y-1 text-sm">
        <div>
          <span class="text-muted-foreground">Insta-buy</span>
          <span class="ml-1 font-mono tabular-nums font-medium">{formatGp(latestHigh || null)}</span>
        </div>
        <div>
          <span class="text-muted-foreground">Insta-sell</span>
          <span class="ml-1 font-mono tabular-nums font-medium">{formatGp(latestLow || null)}</span>
        </div>
        {#if margin != null}
          <div>
            <span class="text-muted-foreground">Margin</span>
            <span
              class="ml-1 font-mono tabular-nums font-medium {margin > 0
                ? 'text-green-500'
                : margin < 0
                  ? 'text-red-500'
                  : ''}">{margin.toLocaleString()} gp</span
            >
          </div>
        {/if}
        {#if item.buyLimit != null}
          <div>
            <span class="text-muted-foreground">Buy limit</span>
            <span class="ml-1 font-mono tabular-nums font-medium"
              >{item.buyLimit.toLocaleString()}</span
            >
          </div>
        {/if}
      </div>
    {/if}
  </div>

  <!-- Signal Strip -->
  {#if signals.length > 0}
    <div class="mb-4 flex flex-wrap gap-2">
      {#each signals as signal}
        <div class="flex items-center gap-3 rounded-lg border px-3 py-2 text-sm">
          <a href={faqAnchor(signal.signalType)}>
            <Badge variant="secondary" class="text-[10px] hover:bg-secondary/80">
              {signalLabel(signal.signalType)}
            </Badge>
          </a>
          <span class="font-mono tabular-nums text-muted-foreground">
            {signal.score.toFixed(2)}
          </span>
          {#if isMomentumSignal(signal.signalType)}
            {#if sma24h != null}
              {@const currentPrice = latestLow || signal.lowPrice}
              {#if currentPrice != null}
                {@const potential = sma24h - currentPrice}
                <span class="font-mono tabular-nums">
                  {currentPrice.toLocaleString()}
                  <span class="text-muted-foreground mx-0.5">&rarr;</span>
                  {sma24h.toLocaleString()} gp
                  <span class={potential >= 0 ? 'text-green-500' : 'text-red-500'}>
                    ({potential >= 0 ? '+' : ''}{potential.toLocaleString()} gp)
                  </span>
                </span>
              {/if}
            {/if}
          {:else if signal.margin != null}
            <span class="font-mono tabular-nums">
              Margin: <span class={signal.margin >= 0 ? 'text-green-500' : 'text-red-500'}>
                {signal.margin.toLocaleString()} gp
              </span>
            </span>
          {/if}
        </div>
      {/each}
    </div>
  {/if}

  <!-- Active Trades -->
  {#if itemTrades.length > 0}
    <div class="mb-4 flex flex-wrap gap-2">
      {#each itemTrades as trade}
        {@const isActive = trade.status === 'active'}
        {@const totalCost = trade.quantity * trade.buyPrice}
        <div
          class="flex items-center gap-3 rounded-lg border px-3 py-2 text-sm {isActive
            ? 'border-primary/30 bg-primary/5'
            : 'border-dashed'}"
        >
          <Badge variant={isActive ? 'default' : 'outline'} class="text-[10px]">
            {isActive ? 'Active trade' : 'Pending buy'}
          </Badge>
          <span class="font-mono tabular-nums">
            {isActive ? 'Bought' : 'Buying'}
            {trade.quantity.toLocaleString()} @ {trade.buyPrice.toLocaleString()} gp
          </span>
          {#if trade.sellPrice != null}
            <span class="font-mono tabular-nums text-muted-foreground">
              target {trade.sellPrice.toLocaleString()} gp
            </span>
            {@const tax = calcGeTax(trade.sellPrice, item.itemId)}
            {@const projected = (trade.sellPrice - tax - trade.buyPrice) * trade.quantity}
            <span class="font-mono italic tabular-nums text-xs text-blue-500">
              ({projected >= 0 ? '+' : ''}{projected.toLocaleString()} gp)
            </span>
          {/if}
        </div>
      {/each}
    </div>
  {/if}

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
        yDomain={priceYDomain}
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
            data={highPriceData}
            y={(d: ChartPoint) => d.highPrice}
            class="stroke-[var(--color-highPrice)] stroke-[1]"
            opacity={0.4}
            stroke-dasharray="2,6"
          />
          <Spline
            y={(d: ChartPoint) => d.highPrice}
            defined={(d: ChartPoint) => d.highPrice != null}
            class="stroke-[var(--color-highPrice)] stroke-[1.5]"
          />
          <Spline
            data={lowPriceData}
            y={(d: ChartPoint) => d.lowPrice}
            class="stroke-[var(--color-lowPrice)] stroke-[1]"
            opacity={0.4}
            stroke-dasharray="2,6"
          />
          <Spline
            y={(d: ChartPoint) => d.lowPrice}
            defined={(d: ChartPoint) => d.lowPrice != null}
            class="stroke-[var(--color-lowPrice)] stroke-[1.5]"
          />
          <Axis placement="bottom" format={axisFormat} tickSpacing={100} />
          <Axis placement="right" format={(v) => v.toLocaleString()} />
          <Highlight points={highlightPoints} lines={highlightLines} />
        </Svg>
        <Tooltip.Root x="data" y="data" anchor="top-right" variant="none" contained="window">
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
                      <span class="size-2.5 rounded-sm bg-[var(--color-highPrice)]"></span>
                      <span class="text-muted-foreground">Insta-Buy</span>
                    </span>
                    <span class="font-mono font-medium tabular-nums">
                      {formatGp(d.highPrice)}
                    </span>
                  </div>
                  <div class="flex items-center justify-between gap-4">
                    <span class="flex items-center gap-1.5">
                      <span class="size-2.5 rounded-sm bg-[var(--color-lowPrice)]"></span>
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
                        {formatVolume((d.highVolume ?? 0) + (d.lowVolume ?? 0))}
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

  <!-- Data Coverage -->
  <details class="mt-8 group">
    <summary
      class="flex cursor-pointer items-center gap-2 text-sm font-medium text-muted-foreground select-none"
    >
      <ChevronDown class="size-4 transition-transform group-open:rotate-180" />
      Data Coverage
    </summary>

    <div class="mt-3 grid gap-3">
      {#each coverage as c (c.bucket)}
        {@const pct = coveragePct(c)}
        <div class="rounded-lg border p-4">
          <div class="flex items-center justify-between mb-2">
            <div class="flex items-center gap-2">
              <span class="font-mono text-sm font-semibold">{c.bucket}</span>
              <span class="text-xs text-muted-foreground">({c.retention} retention)</span>
            </div>
            {#if c.totalBuckets > 0}
              <span
                class="text-xs font-mono tabular-nums {pct >= 95
                  ? 'text-green-500'
                  : pct >= 80
                    ? 'text-yellow-500'
                    : 'text-red-500'}"
              >
                {pct.toFixed(1)}%
              </span>
            {/if}
          </div>

          {#if c.oldest && c.newest}
            <!-- Coverage bar -->
            <div class="mb-3 h-2 w-full rounded-full bg-muted overflow-hidden">
              <div
                class="h-full rounded-full {pct >= 95
                  ? 'bg-green-500'
                  : pct >= 80
                    ? 'bg-yellow-500'
                    : 'bg-red-500'}"
                style="width: {pct}%"
              ></div>
            </div>

            <div class="grid grid-cols-2 gap-x-4 gap-y-1 text-xs">
              <div>
                <span class="text-muted-foreground">First data</span>
                <div class="font-mono tabular-nums">{formatDate(c.oldest)}</div>
              </div>
              <div>
                <span class="text-muted-foreground">Latest data</span>
                <div class="font-mono tabular-nums">
                  {formatDate(c.newest)}
                  <span class="text-muted-foreground ml-1">({dataAge(c.newest)})</span>
                </div>
              </div>
              <div>
                <span class="text-muted-foreground">Buckets</span>
                <div class="font-mono tabular-nums">
                  {c.totalBuckets.toLocaleString()} / {c.expectedBuckets.toLocaleString()}
                </div>
              </div>
              <div>
                <span class="text-muted-foreground">Gaps</span>
                <div class="font-mono tabular-nums">
                  {c.gaps.length === 0 ? 'None' : c.gaps.length}
                </div>
              </div>
            </div>

            {#if c.gaps.length > 0}
              <div class="mt-3 border-t pt-2">
                <div class="text-xs text-muted-foreground mb-1">Recent gaps (newest first)</div>
                <div class="max-h-40 overflow-y-auto space-y-1">
                  {#each c.gaps as gap}
                    <div
                      class="flex items-center justify-between text-xs font-mono tabular-nums rounded px-2 py-1 bg-muted/50"
                    >
                      <span>
                        {formatDate(gap.start)} &rarr; {formatDate(gap.end)}
                      </span>
                      <span class="text-muted-foreground ml-2 whitespace-nowrap">
                        {formatDuration(gap.durationMs)}, {gap.missingCount} missing
                      </span>
                    </div>
                  {/each}
                </div>
              </div>
            {/if}
          {:else}
            <div class="text-xs text-muted-foreground">No data</div>
          {/if}
        </div>
      {/each}
    </div>
  </details>
</div>

<TradeDialog bind:open={tradeDialogOpen} items={[]} prefill={tradeDialogPrefill} />
