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

	function openSignalTrade(signal: ActiveSignal) {
		const sma = signal.metadata?.sma_24h;
		const targetSellPrice = typeof sma === 'number' ? sma : signal.highPrice;
		tradeDialogPrefill = {
			itemId: signal.itemId,
			itemName: signal.itemName,
			itemIcon: signal.itemIcon,
			instaBuyPrice: signal.highPrice,
			instaSellPrice: signal.lowPrice,
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
		| 'volume24h'
		| 'buyLimit'
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
			case 'volume24h':
				return signal.volume24h;
			case 'buyLimit':
				return signal.buyLimit;
			case 'score':
				return signal.score;
		}
	}

	let sorted = $derived.by(() => {
		return data.flipSignals
			.filter((s) => signalFilters.matchesSignal(s))
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
		{ key: 'signalType', label: 'Type' },
		{ key: 'lowPrice', label: 'Buy', filterable: true },
		{ key: 'highPrice', label: 'Sell', filterable: true },
		{ key: 'margin', label: 'Margin', filterable: true },
		{ key: 'marginPct', label: 'Margin %' },
		{ key: 'volume24h', label: 'Volume', filterable: true },
		{ key: 'buyLimit', label: 'Buy Limit', filterable: true },
		{ key: 'score', label: 'Score' }
	];

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

<h1 class="mb-4 text-2xl font-bold">
	Price Signals
	<span class="text-lg font-normal text-muted-foreground">({data.flipSignals.length})</span>
</h1>

{#if sorted.length === 0}
	<p class="text-sm text-muted-foreground">No active price signals right now.</p>
{:else}
	<div class="overflow-x-auto rounded-lg border">
		<table class="w-full text-sm">
			<thead>
				<tr class="border-b bg-muted/50">
					{#each columns as col (col.key)}
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
				{#each sorted as signal (signal.itemId + ':' + signal.signalType)}
					{@const mPct = marginPct(signal)}
					{@const isExpanded = expandedId === signal.itemId}
					<tr
						class="cursor-pointer border-b transition-colors {isExpanded
							? 'bg-muted/30'
							: 'hover:bg-muted/50'}"
						onclick={() => toggleExpand(signal.itemId)}
						role="button"
						tabindex="0"
						onkeydown={(e) => e.key === 'Enter' && toggleExpand(signal.itemId)}
					>
						<td class="px-4 py-3 font-medium">
							<!-- eslint-disable svelte/no-navigation-without-resolve -->
							<a
								href="/items/{signal.itemId}?from=signals"
								class="inline-flex items-center gap-2 hover:underline"
								onclick={(e) => e.stopPropagation()}
							>
								{#if signal.itemIcon}
									<img src={signal.itemIcon} alt="" class="size-5 object-contain" />
								{/if}
								{signal.itemName}
							</a>
							<!-- eslint-enable svelte/no-navigation-without-resolve -->
						</td>
						<td class="px-4 py-3">
							<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
							<a href={faqAnchor(signal.signalType)} onclick={(e) => e.stopPropagation()}>
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
						<td class="px-4 py-3 tabular-nums">{formatVolume(signal.volume24h)}</td>
						<td class="px-4 py-3 tabular-nums">{formatGp(signal.buyLimit)}</td>
						<td class="px-4 py-3 tabular-nums">{signal.score.toFixed(2)}</td>
						<td class="px-4 py-3">
							<button
								type="button"
								class="inline-flex items-center gap-1 rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground transition-colors hover:bg-primary/90"
								onclick={(e) => {
									e.stopPropagation();
									openSignalTrade(signal);
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
								<ItemRowDetail itemId={signal.itemId} />
							</td>
						</tr>
					{/if}
				{/each}
			</tbody>
		</table>
	</div>
{/if}

<TradeDialog bind:open={tradeDialogOpen} items={[]} prefill={tradeDialogPrefill} />
