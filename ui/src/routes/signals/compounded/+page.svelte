<script lang="ts">
  import { Badge } from '$lib/components/ui/badge';
  import ArrowRightLeft from '@lucide/svelte/icons/arrow-right-left';
  import ArrowUpDown from '@lucide/svelte/icons/arrow-up-down';
  import ArrowUp from '@lucide/svelte/icons/arrow-up';
  import ArrowDown from '@lucide/svelte/icons/arrow-down';
  import TradeDialog from '$lib/components/trade-dialog.svelte';
  import ItemRowDetail from '$lib/components/item-row-detail.svelte';
  import ColumnFilterPopover from '$lib/components/column-filter-popover.svelte';
  import type { ActiveSignal } from '$lib/server/db/queries';
  import type { CompoundedItem } from '../+layout.server';
  import { signalFilters } from '$lib/signal-filters.svelte';

  let { data } = $props();

  let expandedId = $state<number | null>(null);

  function toggleExpand(itemId: number) {
    expandedId = expandedId === itemId ? null : itemId;
  }

  let tradeDialogOpen = $state(false);
  let tradeDialogPrefill = $state<{
    itemId: number;
    itemName: string;
    itemIcon: string | null;
    instaBuyPrice: number | null;
    instaSellPrice: number | null;
    targetSellPrice?: number | null;
  } | null>(null);

  function openTrade(item: CompoundedItem) {
    let targetSellPrice: number | null = null;
    for (const s of item.signals) {
      const sma = s.metadata?.sma_24h;
      if (typeof sma === 'number' && sma > 0) {
        targetSellPrice = targetSellPrice != null ? Math.max(targetSellPrice, sma) : sma;
      }
    }
    if (targetSellPrice == null) {
      targetSellPrice = item.highPrice;
    }
    tradeDialogPrefill = {
      itemId: item.itemId,
      itemName: item.itemName,
      itemIcon: item.itemIcon,
      instaBuyPrice: item.highPrice,
      instaSellPrice: item.lowPrice,
      targetSellPrice
    };
    tradeDialogOpen = true;
  }

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

  type SortKey =
    | 'itemName'
    | 'signalCount'
    | 'compositeScore'
    | 'volumeConfidence'
    | 'lowPrice'
    | 'highPrice'
    | 'margin'
    | 'volume24h'
    | 'buyLimit';

  let sortKey = $state<SortKey>('compositeScore');
  let sortDir = $state<'asc' | 'desc'>('desc');

  function toggleSort(key: SortKey) {
    if (sortKey === key) {
      sortDir = sortDir === 'asc' ? 'desc' : 'asc';
    } else {
      sortKey = key;
      sortDir = 'desc';
    }
  }

  function getSortValue(item: CompoundedItem, key: SortKey): string | number | null {
    switch (key) {
      case 'itemName':
        return item.itemName;
      case 'signalCount':
        return item.signals.length;
      case 'compositeScore':
        return item.compositeScore;
      case 'volumeConfidence':
        return item.volumeConfidence;
      case 'lowPrice':
        return item.lowPrice;
      case 'highPrice':
        return item.highPrice;
      case 'margin':
        return item.margin;
      case 'volume24h':
        return item.volume24h;
      case 'buyLimit':
        return item.buyLimit;
    }
  }

  let sorted = $derived.by(() => {
    return (data.compoundedItems as CompoundedItem[])
      .filter((item) => signalFilters.matchesItemSignals(item.signals))
      .toSorted((a, b) => {
        const av = getSortValue(a, sortKey);
        const bv = getSortValue(b, sortKey);
        if (av == null && bv == null) return 0;
        if (av == null) return 1;
        if (bv == null) return -1;
        const cmp = av < bv ? -1 : av > bv ? 1 : 0;
        return sortDir === 'asc' ? cmp : -cmp;
      });
  });

  const columns: { key: SortKey; label: string; filterable?: boolean }[] = [
    { key: 'itemName', label: 'Item' },
    { key: 'signalCount', label: 'Signals' },
    { key: 'compositeScore', label: 'Score' },
    { key: 'volumeConfidence', label: 'RVOL' },
    { key: 'lowPrice', label: 'Buy', filterable: true },
    { key: 'highPrice', label: 'Sell', filterable: true },
    { key: 'margin', label: 'Margin', filterable: true },
    { key: 'volume24h', label: 'Volume', filterable: true },
    { key: 'buyLimit', label: 'Buy Limit', filterable: true }
  ];

  function volColor(vc: number | null): string {
    if (vc == null) return 'text-muted-foreground/40';
    if (vc < 0.5) return 'text-red-500';
    if (vc <= 0.75) return 'text-muted-foreground';
    return 'text-green-500';
  }

  function volLabel(vc: number | null): string {
    if (vc == null) return '—';
    return (vc * 100).toFixed(0) + '%';
  }

  function signalVolIndicator(signal: ActiveSignal): string {
    const vc = signal.metadata?.volume_confidence;
    if (typeof vc !== 'number') return '';
    return vc > 0.5 ? ' ▲' : ' ▼';
  }

  function signalVolColor(signal: ActiveSignal): string {
    const vc = signal.metadata?.volume_confidence;
    if (typeof vc !== 'number') return '';
    return vc > 0.5 ? 'text-green-500' : 'text-red-500';
  }

  function formatGp(n: number | null): string {
    if (n == null) return '—';
    return n.toLocaleString();
  }

  function formatVolume(n: number | null): string {
    if (n == null) return '—';
    if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M';
    if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K';
    return n.toLocaleString();
  }

  const f = signalFilters;
</script>

<h1 class="mb-2 text-2xl font-bold">
  Compounded Signals
  <span class="text-lg font-normal text-muted-foreground"
    >({(data.compoundedItems as CompoundedItem[]).length})</span
  >
</h1>
<p class="mb-4 text-sm text-muted-foreground">
  Items with multiple active signals. When indicators agree, confidence is higher.
</p>

{#if sorted.length === 0}
  <p class="text-sm text-muted-foreground">
    No items with multiple signals right now. Check back when more signals are active.
  </p>
{:else}
  <div class="overflow-x-auto rounded-lg border">
    <table class="w-full text-sm">
      <thead>
        <tr class="border-b bg-muted/50">
          {#each columns as col}
            <th class="px-4 py-3 text-left font-medium text-muted-foreground">
              <div class="inline-flex items-center gap-1">
                <button
                  class="inline-flex cursor-pointer items-center gap-1 select-none"
                  onclick={() => toggleSort(col.key)}
                >
                  {col.label}
                  {#if sortKey === col.key}
                    {#if sortDir === 'asc'}
                      <ArrowUp class="size-3.5" />
                    {:else}
                      <ArrowDown class="size-3.5" />
                    {/if}
                  {:else}
                    <ArrowUpDown class="size-3.5 opacity-30" />
                  {/if}
                </button>
                {#if col.filterable}
                  <ColumnFilterPopover
                    label={col.label}
                    bind:min={f.columnFilters[col.key].min}
                    bind:max={f.columnFilters[col.key].max}
                  />
                {/if}
              </div>
            </th>
          {/each}
          <th class="px-4 py-3"></th>
        </tr>
      </thead>
      <tbody>
        {#each sorted as item}
          {@const isExpanded = expandedId === item.itemId}
          <tr
            class="border-b transition-colors cursor-pointer {isExpanded
              ? 'bg-muted/30'
              : 'hover:bg-muted/50'}"
            onclick={() => toggleExpand(item.itemId)}
            role="button"
            tabindex="0"
            onkeydown={(e) => e.key === 'Enter' && toggleExpand(item.itemId)}
          >
            <td class="px-4 py-3 font-medium">
              <a
                href="/items/{item.itemId}?from=signals"
                class="inline-flex items-center gap-2 hover:underline"
                onclick={(e) => e.stopPropagation()}
              >
                {#if item.itemIcon}
                  <img src={item.itemIcon} alt="" class="size-5 object-contain" />
                {/if}
                {item.itemName}
              </a>
            </td>
            <td class="px-4 py-3">
              <div class="flex flex-wrap gap-1">
                {#each item.signals as signal}
                  <a
                    href={faqAnchor(signal.signalType)}
                    onclick={(e) => e.stopPropagation()}
                  >
                    <Badge variant="secondary" class="text-[10px] hover:bg-secondary/80">
                      {signalLabel(signal.signalType)}
                      <span class="ml-1 opacity-60">{signal.score.toFixed(2)}</span>
                      {#if signalVolIndicator(signal)}
                        <span class="ml-0.5 {signalVolColor(signal)}"
                          >{signalVolIndicator(signal)}</span
                        >
                      {/if}
                    </Badge>
                  </a>
                {/each}
              </div>
            </td>
            <td class="px-4 py-3 tabular-nums">{item.compositeScore.toFixed(2)}</td>
            <td class="px-4 py-3 tabular-nums {volColor(item.volumeConfidence)}"
              >{volLabel(item.volumeConfidence)}</td
            >
            <td class="px-4 py-3 tabular-nums">{formatGp(item.lowPrice)}</td>
            <td class="px-4 py-3 tabular-nums">{formatGp(item.highPrice)}</td>
            <td
              class="px-4 py-3 tabular-nums {item.margin != null && item.margin > 0
                ? 'text-green-500'
                : item.margin != null && item.margin < 0
                  ? 'text-red-500'
                  : ''}"
            >
              {item.margin != null ? formatGp(item.margin) : '—'}
            </td>
            <td class="px-4 py-3 tabular-nums">{formatVolume(item.volume24h)}</td>
            <td class="px-4 py-3 tabular-nums">{formatGp(item.buyLimit)}</td>
            <td class="px-4 py-3">
              <button
                type="button"
                class="inline-flex items-center gap-1 rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground transition-colors hover:bg-primary/90"
                onclick={(e) => {
                  e.stopPropagation();
                  openTrade(item);
                }}
              >
                <ArrowRightLeft class="size-3.5" />
                Flip
              </button>
            </td>
          </tr>
          {#if isExpanded}
            <tr class="bg-muted/30">
              <td colspan={columns.length + 1}>
                <ItemRowDetail itemId={item.itemId} />
              </td>
            </tr>
          {/if}
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<TradeDialog bind:open={tradeDialogOpen} items={[]} prefill={tradeDialogPrefill} />
