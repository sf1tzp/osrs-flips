<script lang="ts">
	import { goto } from '$app/navigation';
	import * as Table from '$lib/components/ui/table';
	import { Input } from '$lib/components/ui/input';
	import { Badge } from '$lib/components/ui/badge';
	import ArrowUpDown from '@lucide/svelte/icons/arrow-up-down';
	import ArrowUp from '@lucide/svelte/icons/arrow-up';
	import ArrowDown from '@lucide/svelte/icons/arrow-down';
	import Search from '@lucide/svelte/icons/search';
	import type { DashboardItem } from '$lib/server/db/queries';

	let { data } = $props();

	type SortKey = keyof DashboardItem;

	let search = $state('');
	let sortKey = $state<SortKey>('marginPct');
	let sortDir = $state<'asc' | 'desc'>('desc');

	function toggleSort(key: SortKey) {
		if (sortKey === key) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			sortKey = key;
			sortDir = 'desc';
		}
	}

	let filtered = $derived.by(() => {
		const q = search.toLowerCase();
		let items = data.items;
		if (q) {
			items = items.filter((i) => i.name.toLowerCase().includes(q));
		}
		return items.toSorted((a, b) => {
			const av = a[sortKey];
			const bv = b[sortKey];
			if (av == null && bv == null) return 0;
			if (av == null) return 1;
			if (bv == null) return -1;
			const cmp = av < bv ? -1 : av > bv ? 1 : 0;
			return sortDir === 'asc' ? cmp : -cmp;
		});
	});

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

	const columns: { key: SortKey; label: string }[] = [
		{ key: 'name', label: 'Item' },
		{ key: 'highPrice', label: 'Insta-Buy' },
		{ key: 'lowPrice', label: 'Insta-Sell' },
		{ key: 'margin', label: 'Margin' },
		{ key: 'marginPct', label: 'Margin %' },
		{ key: 'buyLimit', label: 'Buy Limit' },
		{ key: 'volume24h', label: '24h Volume' },
	];
</script>

<div class="mx-auto max-w-7xl p-4 sm:p-6">
	<div class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<h1 class="text-2xl font-bold">Item Prices</h1>
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

	<div class="rounded-lg border">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					{#each columns as col}
						<Table.Head>
							<button
								class="inline-flex w-full cursor-pointer items-center gap-1 select-none"
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
						</Table.Head>
					{/each}
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each filtered as item (item.itemId)}
					<Table.Row
						class="cursor-pointer"
						onclick={() => goto(`/items/${item.itemId}`)}
					>
						<Table.Cell class="font-medium">
							<span class="inline-flex items-center gap-2">
								<!-- {#if item.icon}
									<img
										src={item.icon}
										alt={item.name}
										loading="lazy"
										class="size-6 object-contain"
									/>
								{/if} -->
								{item.name}
								{#if !item.members}
									<Badge variant="outline" class="text-[10px] leading-tight">Free-To-Play</Badge>
								{/if}
							</span>
						</Table.Cell>
						<Table.Cell class="tabular-nums">{formatGp(item.highPrice)}</Table.Cell>
						<Table.Cell class="tabular-nums">{formatGp(item.lowPrice)}</Table.Cell>
						<Table.Cell class="tabular-nums {marginColor(item.margin)}">
							{formatGp(item.margin)}
						</Table.Cell>
						<Table.Cell class="tabular-nums {marginColor(item.marginPct)}">
							{item.marginPct != null ? `${item.marginPct}%` : '—'}
						</Table.Cell>
						<Table.Cell class="tabular-nums">{formatGp(item.buyLimit)}</Table.Cell>
						<Table.Cell class="tabular-nums">{formatVolume(item.volume24h)}</Table.Cell>
					</Table.Row>
				{/each}
				{#if filtered.length === 0}
					<Table.Row>
						<Table.Cell colspan={7} class="text-muted-foreground h-24 text-center">
							No items found.
						</Table.Cell>
					</Table.Row>
				{/if}
			</Table.Body>
		</Table.Root>
	</div>

	<p class="text-muted-foreground mt-3 text-sm">
		{filtered.length} items
	</p>
</div>
