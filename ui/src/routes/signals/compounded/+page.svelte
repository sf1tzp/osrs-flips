<script lang="ts">
  import { Badge } from '$lib/components/ui/badge';
  import ArrowRightLeft from '@lucide/svelte/icons/arrow-right-left';
  import ArrowUpDown from '@lucide/svelte/icons/arrow-up-down';
  import ArrowUp from '@lucide/svelte/icons/arrow-up';
  import ArrowDown from '@lucide/svelte/icons/arrow-down';
  import TradeDialog from '$lib/components/trade-dialog.svelte';
  import type { ActiveSignal } from '$lib/server/db/queries';
  import type { CompoundedItem } from '../+layout.server';

  let { data } = $props();

  let tradeDialogOpen = $state(false);
  let tradeDialogPrefill = $state<{
    itemId: number;
    itemName: string;
    itemIcon: string | null;
    instaBuyPrice: number | null;
    instaSellPrice: number | null;
  } | null>(null);

  function openTrade(item: CompoundedItem) {
    tradeDialogPrefill = {
      itemId: item.itemId,
      itemName: item.itemName,
      itemIcon: item.itemIcon,
      instaBuyPrice: item.highPrice,
      instaSellPrice: item.lowPrice
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

  type SortKey = 'itemName' | 'signalCount' | 'avgScore' | 'margin';

  let sortKey = $state<SortKey>('signalCount');
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
      case 'avgScore':
        return item.avgScore;
      case 'margin':
        return item.margin;
    }
  }

  let sorted = $derived.by(() => {
    return (data.compoundedItems as CompoundedItem[]).toSorted((a, b) => {
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
    { key: 'signalCount', label: 'Signals' },
    { key: 'avgScore', label: 'Avg Score' },
    { key: 'margin', label: 'Margin' }
  ];

  function formatGp(n: number | null): string {
    if (n == null) return '—';
    return n.toLocaleString();
  }
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
        {#each sorted as item}
          <tr class="border-b transition-colors hover:bg-muted/50">
            <td class="px-4 py-3 font-medium">
              <a
                href="/items/{item.itemId}?from=signals"
                class="inline-flex items-center gap-2 hover:underline"
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
                  <a href={faqAnchor(signal.signalType)}>
                    <Badge variant="secondary" class="text-[10px] hover:bg-secondary/80">
                      {signalLabel(signal.signalType)}
                      <span class="ml-1 opacity-60">{signal.score.toFixed(2)}</span>
                    </Badge>
                  </a>
                {/each}
              </div>
            </td>
            <td class="px-4 py-3 tabular-nums">{item.avgScore.toFixed(2)}</td>
            <td
              class="px-4 py-3 tabular-nums {item.margin != null && item.margin > 0
                ? 'text-green-500'
                : item.margin != null && item.margin < 0
                  ? 'text-red-500'
                  : ''}"
            >
              {item.margin != null ? formatGp(item.margin) : '—'}
            </td>
            <td class="px-4 py-3">
              <button
                type="button"
                class="inline-flex items-center gap-1 rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground transition-colors hover:bg-primary/90"
                onclick={() => openTrade(item)}
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
