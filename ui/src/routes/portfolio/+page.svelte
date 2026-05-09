<script lang="ts">
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { Checkbox } from '$lib/components/ui/checkbox';
  import * as Table from '$lib/components/ui/table';
  import TradeDialog from '$lib/components/trade-dialog.svelte';
  import { tradeStore } from '$lib/portfolio/trade-store.svelte';
  import { aggregatePositions, computeSummary } from '$lib/portfolio/positions';
  import type { DashboardItem } from '$lib/server/db/queries';
  import type { TradePlan } from '$lib/portfolio/types';
  import { calcGeTax } from '$lib/portfolio/types';
  import { parseNumeric } from '$lib/utils';
  import Plus from '@lucide/svelte/icons/plus';
  import Trash2 from '@lucide/svelte/icons/trash-2';

  let { data } = $props();

  // Init trade store on mount
  $effect(() => {
    tradeStore.init();
  });

  // Build price map from server data
  let priceMap = $derived.by(() => {
    const map: Record<number, { highPrice: number | null; lowPrice: number | null }> = {};
    for (const item of data.items) {
      map[item.itemId] = { highPrice: item.highPrice, lowPrice: item.lowPrice };
    }
    return map;
  });

  let aggregation = $derived(aggregatePositions(tradeStore.trades, priceMap));
  let positions = $derived(aggregation.positions);
  let summary = $derived(computeSummary(positions, aggregation.realizedPnl));

  let tradeDialogOpen = $state(false);
  let tradeDialogPrefill = $state<{
    itemId: number;
    itemName: string;
    itemIcon: string | null;
    instaBuyPrice: number | null;
    instaSellPrice: number | null;
  } | null>(null);

  function openNewFlip() {
    tradeDialogPrefill = null;
    tradeDialogOpen = true;
  }

  let deleteConfirm = $state<string | null>(null);

  async function confirmDelete(id: string) {
    if (deleteConfirm === id) {
      await tradeStore.deletePlan(id);
      deleteConfirm = null;
    } else {
      deleteConfirm = id;
      setTimeout(() => {
        if (deleteConfirm === id) deleteConfirm = null;
      }, 3000);
    }
  }

  // Inline editing state
  let editingQty = $state<Record<string, string>>({});
  let editingBuyPrice = $state<Record<string, string>>({});
  let editingSellPrice = $state<Record<string, string>>({});

  async function toggleFilled(plan: TradePlan) {
    if (plan.status === 'pending') {
      await tradeStore.markActive(plan.id);
    } else if (plan.status === 'active') {
      await tradeStore.unfillPlan(plan.id);
    }
  }

  async function toggleSold(plan: TradePlan) {
    if (plan.status === 'active') {
      await closePlan(plan);
    } else if (plan.status === 'closed') {
      await tradeStore.reopenPlan(plan.id);
    }
  }

  async function closePlan(plan: TradePlan) {
    const price = plan.sellPrice || priceMap[plan.itemId]?.highPrice || 0;
    if (price <= 0) return;
    await tradeStore.closePlan(plan.id, price);
  }

  async function commitBuyPrice(plan: TradePlan) {
    const raw = editingBuyPrice[plan.id];
    if (raw == null) return;
    const price = parseNumeric(raw);
    if (price > 0 && price !== plan.buyPrice) {
      await tradeStore.updateBuyPrice(plan.id, price);
    }
    delete editingBuyPrice[plan.id];
  }

  async function commitSellPrice(plan: TradePlan) {
    const raw = editingSellPrice[plan.id];
    if (raw == null) return;
    const price = parseNumeric(raw);
    if (price > 0 && price !== plan.sellPrice) {
      await tradeStore.updateSellPrice(plan.id, price);
    }
    delete editingSellPrice[plan.id];
  }

  async function commitQty(plan: TradePlan) {
    const raw = editingQty[plan.id];
    if (raw == null) return;
    const qty = parseNumeric(raw);
    if (qty > 0 && qty !== plan.quantity) {
      await tradeStore.updateQuantity(plan.id, qty);
    }
    delete editingQty[plan.id];
  }

  function formatGp(n: number | null): string {
    if (n == null) return '--';
    return n.toLocaleString();
  }

  function pnlColor(n: number | null): string {
    if (n == null) return '';
    if (n > 0) return 'text-green-500';
    if (n < 0) return 'text-red-500';
    return '';
  }

  function timeAgo(ts: number): string {
    const ms = Date.now() - ts;
    const mins = Math.floor(ms / 60_000);
    if (mins < 1) return 'just now';
    if (mins < 60) return `${mins}m ago`;
    const hrs = Math.floor(mins / 60);
    if (hrs < 24) return `${hrs}h ago`;
    const days = Math.floor(hrs / 24);
    return `${days}d ago`;
  }

  function planRealizedPnl(plan: TradePlan): number {
    const tax = calcGeTax(plan.sellPrice!, plan.itemId);
    return (plan.sellPrice! - tax - plan.buyPrice) * plan.quantity;
  }
</script>

<div class="mx-auto max-w-7xl p-4 sm:p-6">
  <div class="mb-6 flex items-center justify-between">
    <h1 class="text-2xl font-bold">Portfolio</h1>
    <Button onclick={openNewFlip} size="sm">
      <Plus class="size-4" />
      New Flip
    </Button>
  </div>

  {#if !tradeStore.initialized}
    <!-- Loading skeleton -->
    <div class="grid gap-4 grid-cols-2 lg:grid-cols-4">
      {#each Array(4) as _}
        <div class="rounded-lg border p-4">
          <div class="h-3 w-20 animate-pulse rounded bg-muted"></div>
          <div class="mt-2 h-6 w-28 animate-pulse rounded bg-muted"></div>
        </div>
      {/each}
    </div>
  {:else if tradeStore.trades.length === 0}
    <!-- Empty state -->
    <div class="rounded-lg border p-12 text-center">
      <p class="text-muted-foreground mb-4">No flips yet. Start your first flip!</p>
      <Button onclick={openNewFlip}>
        <Plus class="size-4" />
        New Flip
      </Button>
    </div>
  {:else}
    <!-- Summary cards -->
    <div class="mb-6 grid gap-4 grid-cols-2 lg:grid-cols-4">
      <div class="rounded-lg border p-4">
        <p class="text-xs text-muted-foreground">Active Trade Cost</p>
        <p class="text-xl font-bold tabular-nums">{formatGp(summary.totalCost)} gp</p>
      </div>
      <div class="rounded-lg border p-4">
        <p class="text-xs text-muted-foreground">Unrealized P&L</p>
        <p class="text-xl font-bold tabular-nums {pnlColor(summary.unrealizedPnl)}">
          {summary.unrealizedPnl >= 0 ? '+' : ''}{formatGp(summary.unrealizedPnl)} gp
          {#if summary.unrealizedPnlPct != null}
            <span class="text-sm font-medium">
              ({summary.unrealizedPnlPct >= 0 ? '+' : ''}{summary.unrealizedPnlPct}%)
            </span>
          {/if}
        </p>
      </div>
      <div class=""></div>
      <div class="rounded-lg border p-4">
        <p class="text-xs text-muted-foreground">Realized P&L</p>
        <p class="text-xl font-bold tabular-nums {pnlColor(summary.realizedPnl)}">
          {summary.realizedPnl >= 0 ? '+' : ''}{formatGp(summary.realizedPnl)} gp
        </p>
      </div>
    </div>

    <!-- Positions table -->
    {#if positions.length > 0}
      <div class="mb-8">
        <h2 class="mb-3 text-lg font-semibold">Positions</h2>
        <div class="rounded-lg border">
          <Table.Root>
            <Table.Header>
              <Table.Row>
                <Table.Head>Item</Table.Head>
                <Table.Head class="text-right">Qty</Table.Head>
                <Table.Head class="text-right">Cost</Table.Head>
                <Table.Head class="text-right">Target</Table.Head>
                <Table.Head class="text-right">Current</Table.Head>
                <Table.Head class="text-right">P&L</Table.Head>
                <Table.Head class="text-right">P&L %</Table.Head>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {#each positions as pos (pos.itemId)}
                <Table.Row>
                  <Table.Cell>
                    <a
                      href="/items/{pos.itemId}?from=portfolio"
                      class="inline-flex items-center gap-2 hover:underline"
                    >
                      {#if pos.itemIcon}
                        <img src={pos.itemIcon} alt="" class="size-5 object-contain" />
                      {/if}
                      {pos.itemName}
                    </a>
                  </Table.Cell>
                  <Table.Cell class="text-right tabular-nums">
                    {pos.quantityHeld.toLocaleString()}
                  </Table.Cell>
                  <Table.Cell class="text-right tabular-nums">
                    {formatGp(pos.avgCostBasis)}
                  </Table.Cell>
                  <Table.Cell class="text-right tabular-nums">
                    {formatGp(pos.targetSellPrice)}
                  </Table.Cell>
                  <Table.Cell class="text-right tabular-nums">
                    {formatGp(pos.currentPrice)}
                  </Table.Cell>
                  <Table.Cell class="text-right tabular-nums {pnlColor(pos.unrealizedPnl)}">
                    {pos.unrealizedPnl != null
                      ? `${pos.unrealizedPnl >= 0 ? '+' : ''}${formatGp(pos.unrealizedPnl)}`
                      : '--'}
                  </Table.Cell>
                  <Table.Cell class="text-right tabular-nums {pnlColor(pos.unrealizedPnlPct)}">
                    {pos.unrealizedPnlPct != null
                      ? `${pos.unrealizedPnlPct >= 0 ? '+' : ''}${pos.unrealizedPnlPct}%`
                      : '--'}
                  </Table.Cell>
                </Table.Row>
              {/each}
            </Table.Body>
          </Table.Root>
        </div>
      </div>
    {/if}

    <!-- Flip plans -->
    <div>
      <h2 class="mb-3 text-lg font-semibold">Flip Plans</h2>
      <div class="grid gap-2">
        {#each tradeStore.trades as plan (plan.id)}
          {@const isClosed = plan.status === 'closed'}
          {@const isPending = plan.status === 'pending'}
          {@const filledChecked = plan.status !== 'pending'}
          <div
            class="rounded-lg border px-4 py-3 transition-opacity {isClosed ? 'opacity-60' : ''}"
          >
            <div class="flex items-center gap-3">
              <!-- Item identity -->
              {#if plan.itemIcon}
                <img src={plan.itemIcon} alt="" class="size-5 object-contain" />
              {/if}
              <a
                      href="/items/{plan.itemId}?from=portfolio"
                      class="inline-flex items-center gap-2 hover:underline"
              >
              <span class="text-sm font-medium">{plan.itemName}</span>
              </a>

              <!-- Buy info -->
              <span class="text-muted-foreground text-sm">Buy</span>
              {#if editingQty[plan.id] != null}
                <Input
                  type="text"
                  inputmode="decimal"
                  class="h-6 w-16 text-xs tabular-nums"
                  value={editingQty[plan.id]}
                  oninput={(e) => {
                    editingQty[plan.id] = e.currentTarget.value;
                  }}
                  onblur={() => commitQty(plan)}
                  onkeydown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault();
                      commitQty(plan);
                    }
                    if (e.key === 'Escape') {
                      delete editingQty[plan.id];
                    }
                  }}
                />
              {:else}
                <button
                  type="button"
                  class="text-sm tabular-nums {!isClosed ? 'cursor-pointer hover:underline' : ''}"
                  disabled={isClosed}
                  onclick={() => {
                    if (!isClosed) editingQty[plan.id] = String(plan.quantity);
                  }}
                  title={isClosed ? '' : 'Click to edit quantity'}
                >
                  {plan.quantity.toLocaleString()}
                </button>
              {/if}
              <span class="text-muted-foreground text-sm">@</span>
              {#if editingBuyPrice[plan.id] != null}
                <Input
                  type="text"
                  inputmode="decimal"
                  class="h-6 w-20 text-xs tabular-nums"
                  value={editingBuyPrice[plan.id]}
                  oninput={(e) => {
                    editingBuyPrice[plan.id] = e.currentTarget.value;
                  }}
                  onblur={() => commitBuyPrice(plan)}
                  onkeydown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault();
                      commitBuyPrice(plan);
                    }
                    if (e.key === 'Escape') {
                      delete editingBuyPrice[plan.id];
                    }
                  }}
                />
              {:else}
                <button
                  type="button"
                  class="text-sm tabular-nums {!isClosed ? 'cursor-pointer hover:underline' : ''}"
                  disabled={isClosed}
                  onclick={() => {
                    if (!isClosed) editingBuyPrice[plan.id] = String(plan.buyPrice);
                  }}
                  title={isClosed ? '' : 'Click to edit buy price'}
                >
                  {plan.buyPrice.toLocaleString()} gp
                </button>
              {/if}

              <!-- Filled checkbox -->
              <label
                class="ml-2 flex items-center gap-1.5 text-sm {isPending
                  ? 'text-muted-foreground'
                  : ''}"
              >
                <Checkbox
                  checked={filledChecked}
                  disabled={isClosed}
                  onCheckedChange={() => toggleFilled(plan)}
                />
                Filled
              </label>

              <!-- Sell side -->
              <span class="text-muted-foreground text-sm ml-2">Sell @</span>
              {#if editingSellPrice[plan.id] != null}
                <Input
                  type="text"
                  inputmode="decimal"
                  class="h-6 w-20 text-xs tabular-nums"
                  placeholder={priceMap[plan.itemId]?.highPrice != null
                    ? String(priceMap[plan.itemId].highPrice)
                    : 'price'}
                  value={editingSellPrice[plan.id]}
                  oninput={(e) => {
                    editingSellPrice[plan.id] = e.currentTarget.value;
                  }}
                  onblur={() => commitSellPrice(plan)}
                  onkeydown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault();
                      commitSellPrice(plan);
                    }
                    if (e.key === 'Escape') {
                      delete editingSellPrice[plan.id];
                    }
                  }}
                />
              {:else}
                <button
                  type="button"
                  class="text-sm tabular-nums {!isClosed
                    ? 'cursor-pointer hover:underline'
                    : ''}"
                  disabled={isClosed}
                  onclick={() => {
                    if (!isClosed)
                      editingSellPrice[plan.id] =
                        plan.sellPrice != null ? String(plan.sellPrice) : '';
                  }}
                  title={isClosed ? '' : 'Click to set sell price'}
                >
                  {plan.sellPrice != null
                    ? `${plan.sellPrice.toLocaleString()} gp`
                    : priceMap[plan.itemId]?.highPrice != null
                      ? `${priceMap[plan.itemId].highPrice!.toLocaleString()} gp`
                      : '--'}
                </button>
              {/if}
              <label
                class="flex items-center gap-1.5 text-sm {isClosed ? '' : 'text-muted-foreground'}"
              >
                <Checkbox
                  checked={isClosed}
                  disabled={!isClosed &&
                    (plan.sellPrice || priceMap[plan.itemId]?.highPrice || 0) <= 0}
                  onCheckedChange={() => toggleSold(plan)}
                />
                Sold
              </label>
              {#if isClosed}
                {@const pnl = planRealizedPnl(plan)}
                <span class="text-sm font-medium {pnlColor(pnl)}">
                  {pnl >= 0 ? '+' : ''}{formatGp(pnl)} gp
                </span>
              {:else if plan.sellPrice != null && plan.sellPrice > 0}
                {@const tax = calcGeTax(plan.sellPrice, plan.itemId)}
                {@const projected = (plan.sellPrice - tax - plan.buyPrice) * plan.quantity}
                <span class="text-sm italic font-medium text-blue-500">
                  ({projected >= 0 ? '+' : '-'}{formatGp(projected)} gp)
                </span>
              {/if}

              <!-- Spacer + actions -->
              <div class="ml-auto flex items-center gap-2">
                <span class="text-xs text-muted-foreground">{timeAgo(plan.createdAt)}</span>
                <button
                  type="button"
                  class="text-muted-foreground hover:text-destructive transition-colors"
                  title={deleteConfirm === plan.id ? 'Click again to confirm' : 'Delete flip'}
                  onclick={() => confirmDelete(plan.id)}
                >
                  <Trash2 class="size-4 {deleteConfirm === plan.id ? 'text-destructive' : ''}" />
                </button>
              </div>
            </div>
          </div>
        {/each}
      </div>
    </div>
  {/if}
</div>

<TradeDialog bind:open={tradeDialogOpen} items={data.items} prefill={tradeDialogPrefill} />
