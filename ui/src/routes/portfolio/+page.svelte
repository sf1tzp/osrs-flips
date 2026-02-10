<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import * as Table from '$lib/components/ui/table';
	import TradeDialog from '$lib/components/trade-dialog.svelte';
	import { tradeStore } from '$lib/portfolio/trade-store.svelte';
	import { aggregatePositions, computeSummary } from '$lib/portfolio/positions';
	import type { DashboardItem } from '$lib/server/db/queries';
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

	let positions = $derived(aggregatePositions(tradeStore.trades, priceMap));
	let summary = $derived(computeSummary(positions));
	let recentTrades = $derived(tradeStore.trades.slice(0, 20));

	let tradeDialogOpen = $state(false);
	let tradeDialogPrefill = $state<{
		itemId: number;
		itemName: string;
		itemIcon: string | null;
		suggestedPrice: number;
		suggestedType: 'buy' | 'sell';
	} | null>(null);

	function openNewTrade() {
		tradeDialogPrefill = null;
		tradeDialogOpen = true;
	}

	let deleteConfirm = $state<string | null>(null);

	async function confirmDelete(id: string) {
		if (deleteConfirm === id) {
			await tradeStore.deleteTrade(id);
			deleteConfirm = null;
		} else {
			deleteConfirm = id;
			setTimeout(() => {
				if (deleteConfirm === id) deleteConfirm = null;
			}, 3000);
		}
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
</script>

<div class="mx-auto max-w-7xl p-4 sm:p-6">
	<div class="mb-6 flex items-center justify-between">
		<h1 class="text-2xl font-bold">Portfolio</h1>
		<Button onclick={openNewTrade} size="sm">
			<Plus class="size-4" />
			New Trade
		</Button>
	</div>

	{#if !tradeStore.initialized}
		<!-- Loading skeleton -->
		<div class="grid gap-4 sm:grid-cols-3">
			{#each Array(3) as _}
				<div class="rounded-lg border p-4">
					<div class="h-3 w-20 animate-pulse rounded bg-muted"></div>
					<div class="mt-2 h-6 w-28 animate-pulse rounded bg-muted"></div>
				</div>
			{/each}
		</div>
	{:else if tradeStore.trades.length === 0}
		<!-- Empty state -->
		<div class="rounded-lg border p-12 text-center">
			<p class="text-muted-foreground mb-4">No positions yet. Add your first trade!</p>
			<Button onclick={openNewTrade}>
				<Plus class="size-4" />
				Add Trade
			</Button>
		</div>
	{:else}
		<!-- Summary cards -->
		<div class="mb-6 grid gap-4 sm:grid-cols-3">
			<div class="rounded-lg border p-4">
				<p class="text-xs text-muted-foreground">Total Value</p>
				<p class="text-xl font-bold tabular-nums">{formatGp(summary.totalValue)} gp</p>
			</div>
			<div class="rounded-lg border p-4">
				<p class="text-xs text-muted-foreground">Total Cost</p>
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
								<Table.Head class="text-right">Avg Cost</Table.Head>
								<Table.Head class="text-right">Current</Table.Head>
								<Table.Head class="text-right">Value</Table.Head>
								<Table.Head class="text-right">P&L</Table.Head>
								<Table.Head class="text-right">P&L %</Table.Head>
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each positions as pos (pos.itemId)}
								<Table.Row>
									<Table.Cell>
										<a
											href="/items/{pos.itemId}"
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
										{formatGp(pos.currentPrice)}
									</Table.Cell>
									<Table.Cell class="text-right tabular-nums">
										{formatGp(pos.currentValue)}
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

		<!-- Recent trades -->
		<div>
			<h2 class="mb-3 text-lg font-semibold">Recent Trades</h2>
			<div class="grid gap-2">
				{#each recentTrades as trade (trade.id)}
					<div
						class="flex items-center justify-between rounded-lg border px-4 py-3"
					>
						<div class="flex items-center gap-3">
							<Badge variant={trade.type === 'buy' ? 'default' : 'outline'}>
								{trade.type === 'buy' ? 'BUY' : 'SELL'}
							</Badge>
							{#if trade.itemIcon}
								<img src={trade.itemIcon} alt="" class="size-5 object-contain" />
							{/if}
							<div>
								<span class="text-sm font-medium">{trade.itemName}</span>
								<span class="text-muted-foreground text-sm">
									&middot; {trade.quantity.toLocaleString()} @ {trade.pricePerUnit.toLocaleString()} gp
								</span>
							</div>
						</div>
						<div class="flex items-center gap-3">
							<span class="text-xs text-muted-foreground">{timeAgo(trade.timestamp)}</span>
							<button
								type="button"
								class="text-muted-foreground hover:text-destructive transition-colors"
								title={deleteConfirm === trade.id ? 'Click again to confirm' : 'Delete trade'}
								onclick={() => confirmDelete(trade.id)}
							>
								<Trash2 class="size-4 {deleteConfirm === trade.id ? 'text-destructive' : ''}" />
							</button>
						</div>
					</div>
				{/each}
			</div>
		</div>
	{/if}
</div>

<TradeDialog
	bind:open={tradeDialogOpen}
	items={data.items}
	prefill={tradeDialogPrefill}
/>
