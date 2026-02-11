<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import ArrowRightLeft from '@lucide/svelte/icons/arrow-right-left';
	import TrendingUp from '@lucide/svelte/icons/trending-up';
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

	function marginPctFromSignal(signal: ActiveSignal): number | null {
		if (signal.highPrice == null || signal.lowPrice == null || signal.lowPrice === 0) return null;
		if (signal.margin == null) return null;
		return Math.round((signal.margin / signal.lowPrice) * 1000) / 10;
	}
</script>

<div class="mx-auto max-w-7xl p-4 sm:p-6">
	<h1 class="mb-4 flex items-center gap-2 text-2xl font-bold">
		<TrendingUp class="size-6 text-green-500" />
		Flip Opportunities
		<span class="text-lg font-normal text-muted-foreground">({data.signals.length})</span>
	</h1>

	{#if data.signals.length === 0}
		<p class="text-muted-foreground text-sm">No active signals right now.</p>
	{:else}
		<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
			{#each data.signals as signal}
				{@const mPct = marginPctFromSignal(signal)}
				<div class="flex flex-col gap-2 rounded-lg border p-4 transition-colors hover:bg-muted/50">
					<div class="flex items-center justify-between">
						<div class="flex items-center gap-2">
							{#if signal.itemIcon}
								<img src={signal.itemIcon} alt="" class="size-5 object-contain" />
							{/if}
							<a href="/items/{signal.itemId}" class="text-sm font-medium hover:underline">
								{signal.itemName}
							</a>
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
	{/if}
</div>

<TradeDialog
	bind:open={tradeDialogOpen}
	items={[]}
	prefill={tradeDialogPrefill}
/>
