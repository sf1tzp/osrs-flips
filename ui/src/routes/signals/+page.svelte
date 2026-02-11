<script lang="ts">
  import { Badge } from '$lib/components/ui/badge';
  import ArrowRightLeft from '@lucide/svelte/icons/arrow-right-left';
  import ArrowUpDown from '@lucide/svelte/icons/arrow-up-down';
  import ArrowUp from '@lucide/svelte/icons/arrow-up';
  import ArrowDown from '@lucide/svelte/icons/arrow-down';
  import TradeDialog from '$lib/components/trade-dialog.svelte';
  import type { ActiveSignal } from '$lib/server/db/queries';

  let { data } = $props();

  let tradeDialogOpen = $state(false);
  let tradeDialogPrefill = $state<{
    itemId: number;
    itemName: string;
    itemIcon: string | null;
    instaBuyPrice: number | null;
    instaSellPrice: number | null;
  } | null>(null);

  function openSignalTrade(signal: ActiveSignal) {
    tradeDialogPrefill = {
      itemId: signal.itemId,
      itemName: signal.itemName,
      itemIcon: signal.itemIcon,
      instaBuyPrice: signal.highPrice,
      instaSellPrice: signal.lowPrice
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

  function marginPct(signal: ActiveSignal): number | null {
    if (signal.highPrice == null || signal.lowPrice == null || signal.lowPrice === 0) return null;
    if (signal.margin == null) return null;
    return Math.round((signal.margin / signal.lowPrice) * 1000) / 10;
  }

  type SortKey =
    | 'itemName'
    | 'signalType'
    | 'lowPrice'
    | 'highPrice'
    | 'margin'
    | 'marginPct'
    | 'score';

  let sortKey = $state<SortKey>('margin');
  let sortDir = $state<'asc' | 'desc'>('desc');

  function toggleSort(key: SortKey) {
    if (sortKey === key) {
      sortDir = sortDir === 'asc' ? 'desc' : 'asc';
    } else {
      sortKey = key;
      sortDir = 'desc';
    }
  }

  function getSortValue(signal: ActiveSignal, key: SortKey): string | number | null {
    switch (key) {
      case 'itemName':
        return signal.itemName;
      case 'signalType':
        return signal.signalType;
      case 'lowPrice':
        return signal.lowPrice;
      case 'highPrice':
        return signal.highPrice;
      case 'margin':
        return signal.margin;
      case 'marginPct':
        return marginPct(signal);
      case 'score':
        return signal.score;
    }
  }

  let sorted = $derived.by(() => {
    return data.flipSignals.toSorted((a, b) => {
      const av = getSortValue(a, sortKey);
      const bv = getSortValue(b, sortKey);
      if (av == null && bv == null) return 0;
      if (av == null) return 1;
      if (bv == null) return -1;
      const cmp = av < bv ? -1 : av > bv ? 1 : 0;
      return sortDir === 'asc' ? cmp : -cmp;
    });
  });

  const columns: { key: SortKey; label: string }[] = [
    { key: 'itemName', label: 'Item' },
    { key: 'signalType', label: 'Type' },
    { key: 'lowPrice', label: 'Buy' },
    { key: 'highPrice', label: 'Sell' },
    { key: 'margin', label: 'Margin' },
    { key: 'marginPct', label: 'Margin %' },
    { key: 'score', label: 'Score' }
  ];

  function formatGp(n: number | null): string {
    if (n == null) return '—';
    return n.toLocaleString();
  }
</script>

<h1 class="mb-4 text-2xl font-bold">
  Flip Signals
  <span class="text-lg font-normal text-muted-foreground">({data.flipSignals.length})</span>
</h1>

{#if sorted.length === 0}
  <p class="text-sm text-muted-foreground">No active flip signals right now.</p>
{:else}
  <div class="overflow-x-auto rounded-lg border">
    <table class="w-full text-sm">
      <thead>
        <tr class="border-b bg-muted/50">
          {#each columns as col}
            <th class="px-4 py-3 text-left font-medium text-muted-foreground">
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
            </th>
          {/each}
          <th class="px-4 py-3"></th>
        </tr>
      </thead>
      <tbody>
        {#each sorted as signal}
          {@const mPct = marginPct(signal)}
          <tr class="border-b transition-colors hover:bg-muted/50">
            <td class="px-4 py-3 font-medium">
              <a
                href="/items/{signal.itemId}?from=signals"
                class="inline-flex items-center gap-2 hover:underline"
              >
                {#if signal.itemIcon}
                  <img src={signal.itemIcon} alt="" class="size-5 object-contain" />
                {/if}
                {signal.itemName}
              </a>
            </td>
            <td class="px-4 py-3">
              <a href={faqAnchor(signal.signalType)}>
                <Badge variant="secondary" class="text-[10px] hover:bg-secondary/80">
                  {signalLabel(signal.signalType)}
                </Badge>
              </a>
            </td>
            <td class="px-4 py-3 tabular-nums">{formatGp(signal.lowPrice)}</td>
            <td class="px-4 py-3 tabular-nums">{formatGp(signal.highPrice)}</td>
            <td
              class="px-4 py-3 tabular-nums {signal.margin != null && signal.margin > 0
                ? 'text-green-500'
                : signal.margin != null && signal.margin < 0
                  ? 'text-red-500'
                  : ''}"
            >
              {formatGp(signal.margin)}
            </td>
            <td
              class="px-4 py-3 tabular-nums {mPct != null && mPct > 0
                ? 'text-green-500'
                : mPct != null && mPct < 0
                  ? 'text-red-500'
                  : ''}"
            >
              {mPct != null ? `${mPct}%` : '—'}
            </td>
            <td class="px-4 py-3 tabular-nums">{signal.score.toFixed(2)}</td>
            <td class="px-4 py-3">
              <button
                type="button"
                class="inline-flex items-center gap-1 rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground transition-colors hover:bg-primary/90"
                onclick={() => openSignalTrade(signal)}
              >
                <ArrowRightLeft class="size-3.5" />
                Flip
              </button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<TradeDialog bind:open={tradeDialogOpen} items={[]} prefill={tradeDialogPrefill} />
