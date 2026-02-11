<script lang="ts">
	import { goto } from '$app/navigation';
	import { Input } from '$lib/components/ui/input';
	import { Badge } from '$lib/components/ui/badge';
	import { Switch } from '$lib/components/ui/switch';
	import { VList } from 'virtua/svelte';
	import ArrowUpDown from '@lucide/svelte/icons/arrow-up-down';
	import ArrowUp from '@lucide/svelte/icons/arrow-up';
	import ArrowDown from '@lucide/svelte/icons/arrow-down';
	import Search from '@lucide/svelte/icons/search';
	import ArrowRightLeft from '@lucide/svelte/icons/arrow-right-left';
	import ItemRowDetail from '$lib/components/item-row-detail.svelte';
	import TradeDialog from '$lib/components/trade-dialog.svelte';
	import ColumnFilterPopover from '$lib/components/column-filter-popover.svelte';
	import { parseNumeric } from '$lib/utils';
	import type { DashboardItem, ActiveSignal } from '$lib/server/db/queries';
	import TrendingUp from '@lucide/svelte/icons/trending-up';

	let { data } = $props();

	let tradeDialogOpen = $state(false);
	let tradeDialogPrefill = $state<{
		itemId: number;
		itemName: string;
		itemIcon: string | null;
		instaBuyPrice: number | null;
		instaSellPrice: number | null;
	} | null>(null);

	function openQuickTrade(item: DashboardItem) {
		tradeDialogPrefill = {
			itemId: item.itemId,
			itemName: item.name,
			itemIcon: item.icon,
			instaBuyPrice: item.highPrice,
			instaSellPrice: item.lowPrice
		};
		tradeDialogOpen = true;
	}

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

	function marginPctFromSignal(signal: ActiveSignal): number | null {
		if (signal.highPrice == null || signal.lowPrice == null || signal.lowPrice === 0) return null;
		if (signal.margin == null) return null;
		return Math.round((signal.margin / signal.lowPrice) * 1000) / 10;
	}

	type SortKey = keyof DashboardItem;

	let search = $state('');
	let sortKey = $state<SortKey>('marginPct');
	let sortDir = $state<'asc' | 'desc'>('desc');
	let showTax = $state(true);
	let columnFilters = $state<Record<string, { min: string; max: string }>>({
		highPrice: { min: '', max: '' },
		lowPrice: { min: '', max: '' },
		margin: { min: '', max: '' },
		marginPct: { min: '', max: '' },
		buyLimit: { min: '', max: '' },
		volume24h: { min: '', max: '' },
	});
	let expandedId = $state<number | null>(null);

	function toggleExpand(itemId: number) {
		expandedId = expandedId === itemId ? null : itemId;
	}

	function calcTax(highPrice: number): number {
		return Math.min(Math.floor(highPrice * 0.02), 5_000_000);
	}

	function getMargin(item: DashboardItem): number | null {
		if (item.highPrice == null || item.lowPrice == null) return null;
		let raw = item.highPrice - item.lowPrice;
		if (item.itemId == 13190) raw = raw - .1 * item.highPrice; // oldchool bonds require an additional 10% fee
		return showTax ? raw - calcTax(item.highPrice) : raw;
	}

	function getMarginPct(item: DashboardItem): number | null {
		if (item.highPrice == null || item.lowPrice == null || item.lowPrice === 0) return null;
		const margin = getMargin(item)!;
		return Math.round((margin / item.lowPrice) * 1000) / 10;
	}

	function toggleSort(key: SortKey) {
		if (sortKey === key) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			sortKey = key;
			sortDir = 'desc';
		}
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

	function marginColor(n: number | null): string {
		if (n == null) return '';
		if (n > 0) return 'text-green-500';
		if (n < 0) return 'text-red-500';
		return '';
	}

	const columns: { key: SortKey; label: string; filterable?: boolean }[] = [
		{ key: 'name', label: 'Item' },
		{ key: 'highPrice', label: 'Insta-Buy', filterable: true },
		{ key: 'lowPrice', label: 'Insta-Sell', filterable: true },
		{ key: 'margin', label: 'Margin', filterable: true },
		{ key: 'marginPct', label: 'Margin %', filterable: true },
		{ key: 'buyLimit', label: 'Buy Limit', filterable: true },
		{ key: 'volume24h', label: '24h Volume', filterable: true },
	];

	function getColumnValue(item: DashboardItem, key: SortKey): number | null {
		if (key === 'margin') return getMargin(item);
		if (key === 'marginPct') return getMarginPct(item);
		const v = item[key];
		return typeof v === 'number' ? v : null;
	}

	let filtered = $derived.by(() => {
		const q = search.toLowerCase();
		let items = data.items;
		if (q) {
			items = items.filter((i) => i.name.toLowerCase().includes(q));
		}
		for (const col of columns) {
			if (!col.filterable) continue;
			const f = columnFilters[col.key];
			if (!f) continue;
			const minVal = parseNumeric(f.min);
			const maxVal = parseNumeric(f.max);
			if (minVal > 0 || maxVal > 0) {
				items = items.filter((i) => {
					const v = getColumnValue(i, col.key);
					if (v == null) return false;
					if (minVal > 0 && v < minVal) return false;
					if (maxVal > 0 && v > maxVal) return false;
					return true;
				});
			}
		}
		return items.toSorted((a, b) => {
			let av: string | number | boolean | null;
			let bv: string | number | boolean | null;
			if (sortKey === 'name') {
				av = a[sortKey];
				bv = b[sortKey];
			} else {
				av = getColumnValue(a, sortKey);
				bv = getColumnValue(b, sortKey);
			}
			if (av == null && bv == null) return 0;
			if (av == null) return 1;
			if (bv == null) return -1;
			const cmp = av < bv ? -1 : av > bv ? 1 : 0;
			return sortDir === 'asc' ? cmp : -cmp;
		});
	});
</script>

<div class="mx-auto max-w-7xl p-4 sm:p-6">
	{#if data.signals.length > 0}
		<div class="mb-6">
			<h2 class="mb-3 flex items-center gap-2 text-lg font-semibold">
				<TrendingUp class="size-5 text-green-500" />
				Flip Opportunities
			</h2>
			<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
				{#each data.signals as signal}
					{@const mPct = marginPctFromSignal(signal)}
					<div class="flex flex-col gap-2 rounded-lg border p-4 transition-colors hover:bg-muted/50">
						<div class="flex items-center justify-between">
							<div class="flex items-center gap-2">
								{#if signal.itemIcon}
									<img src={signal.itemIcon} alt="" class="size-5 object-contain" />
								{/if}
								<span class="text-sm font-medium">{signal.itemName}</span>
							</div>
							<Badge variant="secondary" class="text-[10px]">
								{signalLabel(signal.signalType)} &middot; {signal.score.toFixed(2)}
							</Badge>
						</div>
						<div class="flex items-center justify-between text-xs text-muted-foreground">
							<span>Buy: {signal.lowPrice != null ? signal.lowPrice.toLocaleString() : '—'}</span>
							<span>Sell: {signal.highPrice != null ? signal.highPrice.toLocaleString() : '—'}</span>
							<span class="font-medium {signal.margin != null && signal.margin > 0 ? 'text-green-500' : ''}">
								{signal.margin != null ? signal.margin.toLocaleString() : '—'}
								{#if mPct != null}({mPct}%){/if}
							</span>
						</div>
						<button
							type="button"
							class="mt-1 inline-flex w-full items-center justify-center gap-1 rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground transition-colors hover:bg-primary/90"
							onclick={() => openSignalTrade(signal)}
						>
							<ArrowRightLeft class="size-3.5" />
							Flip
						</button>
					</div>
				{/each}
			</div>
		</div>
	{/if}

	<div class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<h1 class="text-2xl font-bold">Item Prices</h1>
		<div class="flex flex-wrap items-center gap-3">
			<label class="flex items-center gap-2 text-sm">
				<Switch bind:checked={showTax} />
				<span class="text-muted-foreground">GE Tax</span>
			</label>
			<div class="relative w-full sm:w-72">
				<Search class="text-muted-foreground absolute left-2.5 top-2.5 size-4" />
				<Input
					type="text"
					placeholder="Search items..."
					class="pl-9"
					bind:value={search}
				/>
			</div>
		</div>
	</div>

	<div class="rounded-lg border">
		<!-- Header -->
		<div class="grid grid-cols-[2fr_1fr_1fr_1fr_1fr_1fr_1fr] border-b bg-muted/50">
			{#each columns as col}
				<div class="px-4 py-3 text-sm font-medium text-muted-foreground">
					<div class="inline-flex w-full items-center gap-1">
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
								bind:min={columnFilters[col.key].min}
								bind:max={columnFilters[col.key].max}
							/>
						{/if}
					</div>
				</div>
			{/each}
		</div>

		<!-- Virtualized rows -->
		{#if filtered.length === 0}
			<div class="text-muted-foreground flex h-24 items-center justify-center text-sm">
				No items found.
			</div>
		{:else}
			<VList data={filtered} style="height: calc(100vh - 200px);" getKey={(item) => item.itemId}>
				{#snippet children(item: DashboardItem)}
					{@const margin = getMargin(item)}
					{@const marginPct = getMarginPct(item)}
					{@const isExpanded = expandedId === item.itemId}
					<div class="border-b {isExpanded ? 'bg-muted/30' : ''}">
						<div
							class="grid cursor-pointer grid-cols-[2fr_1fr_1fr_1fr_1fr_1fr_1fr] transition-colors hover:bg-muted/50"
							onclick={() => toggleExpand(item.itemId)}
							role="button"
							tabindex="0"
							onkeydown={(e) => e.key === 'Enter' && toggleExpand(item.itemId)}
						>
							<div class="px-4 py-3 text-sm font-medium">
								<a
									href="/items/{item.itemId}"
									class="inline-flex items-center gap-2 hover:underline"
									onclick={(e) => e.stopPropagation()}
								>
									{item.name}
									{#if !item.members}
										<Badge variant="outline" class="text-[10px] leading-tight">Free-To-Play</Badge>
									{/if}
								</a>
							</div>
							<div class="px-4 py-3 text-sm tabular-nums">{formatGp(item.highPrice)}</div>
							<div class="px-4 py-3 text-sm tabular-nums">{formatGp(item.lowPrice)}</div>
							<div class="px-4 py-3 text-sm tabular-nums {marginColor(margin)}">
								{formatGp(margin)}
							</div>
							<div class="px-4 py-3 text-sm tabular-nums {marginColor(marginPct)}">
								{marginPct != null ? `${marginPct}%` : '—'}
							</div>
							<div class="px-4 py-3 text-sm tabular-nums">{formatGp(item.buyLimit)}</div>
							<div class="px-4 py-3 text-sm tabular-nums">{formatVolume(item.volume24h)}</div>
						</div>
						{#if isExpanded}
							<div class="flex items-center">
								<ItemRowDetail itemId={item.itemId} />
								<div class="ml-auto flex gap-2 px-4">
									<button
										type="button"
										class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
										title="Quick trade"
										onclick={(e) => { e.stopPropagation(); openQuickTrade(item); }}
									>
										<ArrowRightLeft class="size-3.5" />
										Trade
									</button>
								</div>
							</div>
						{/if}
					</div>
				{/snippet}
			</VList>
		{/if}
	</div>

	<p class="text-muted-foreground mt-3 text-sm">
		{filtered.length} items
	</p>
</div>

<TradeDialog
	bind:open={tradeDialogOpen}
	items={data.items}
	prefill={tradeDialogPrefill}
/>
