<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog';
	import { Label } from '$lib/components/ui/label';
	import { Input } from '$lib/components/ui/input';
	import { Button } from '$lib/components/ui/button';
	import ItemPicker from '$lib/components/item-picker.svelte';
	import { tradeStore } from '$lib/portfolio/trade-store.svelte';
	import { calcGeTax } from '$lib/portfolio/types';
	import type { Trade } from '$lib/portfolio/types';
	import { parseNumeric } from '$lib/utils';
	import type { DashboardItem } from '$lib/server/db/queries';

	interface Prefill {
		itemId: number;
		itemName: string;
		itemIcon: string | null;
		suggestedPrice: number;
		suggestedType: 'buy' | 'sell';
	}

	interface Props {
		open: boolean;
		items: DashboardItem[];
		prefill?: Prefill | null;
		editTrade?: Trade | null;
	}

	let { open = $bindable(false), items, prefill = null, editTrade = null }: Props = $props();

	let isEditing = $derived(editTrade != null);

	let selectedItem = $state<{ id: number; name: string; icon: string | null } | null>(null);
	let tradeType = $state<'buy' | 'sell'>('buy');
	let quantity = $state('');
	let pricePerUnit = $state('');
	let notes = $state('');

	// Reset form when dialog opens
	$effect(() => {
		if (open) {
			if (editTrade) {
				selectedItem = { id: editTrade.itemId, name: editTrade.itemName, icon: editTrade.itemIcon };
				tradeType = editTrade.type;
				quantity = String(editTrade.quantity);
				pricePerUnit = String(editTrade.pricePerUnit);
				notes = editTrade.notes;
			} else if (prefill) {
				selectedItem = { id: prefill.itemId, name: prefill.itemName, icon: prefill.itemIcon };
				tradeType = prefill.suggestedType;
				pricePerUnit = String(prefill.suggestedPrice);
				quantity = '';
				notes = '';
			} else {
				selectedItem = null;
				tradeType = 'buy';
				pricePerUnit = '';
				quantity = '';
				notes = '';
			}
		}
	});

	let price = $derived(parseNumeric(pricePerUnit));
	let qty = $derived(parseNumeric(quantity));
	let tax = $derived(
		tradeType === 'sell' && selectedItem && price > 0
			? calcGeTax(price, selectedItem.id)
			: 0
	);
	let netPerUnit = $derived(tradeType === 'sell' ? price - tax : price);
	let totalCost = $derived(qty * netPerUnit);

	let valid = $derived(selectedItem != null && qty > 0 && price > 0);

	async function submit() {
		if (!valid || !selectedItem) return;

		const trade: Trade = {
			id: editTrade ? editTrade.id : Math.random().toString(36).slice(2) + Date.now().toString(36),
			itemId: selectedItem.id,
			itemName: selectedItem.name,
			itemIcon: selectedItem.icon,
			type: tradeType,
			quantity: qty,
			pricePerUnit: price,
			timestamp: editTrade ? editTrade.timestamp : Date.now(),
			notes: notes.trim()
		};

		await tradeStore.addTrade(trade);
		open = false;
	}

	function onItemSelect(item: DashboardItem) {
		selectedItem = { id: item.itemId, name: item.name, icon: item.icon };
		// Pre-fill price from market data
		if (tradeType === 'buy' && item.lowPrice != null) {
			pricePerUnit = String(item.lowPrice);
		} else if (tradeType === 'sell' && item.highPrice != null) {
			pricePerUnit = String(item.highPrice);
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{isEditing ? 'Edit Trade' : 'New Trade'}</Dialog.Title>
			<Dialog.Description>{isEditing ? 'Update trade details.' : 'Log a buy or sell trade.'}</Dialog.Description>
		</Dialog.Header>

		<form
			class="grid gap-4 py-2"
			onsubmit={(e) => {
				e.preventDefault();
				submit();
			}}
		>
			<!-- Item -->
			<div class="grid gap-2">
				<Label>Item</Label>
				{#if (prefill || isEditing) && selectedItem}
					<div class="flex items-center gap-2 rounded-md border px-3 py-2 text-sm">
						{#if selectedItem.icon}
							<img src={selectedItem.icon} alt="" class="size-5 object-contain" />
						{/if}
						<span>{selectedItem.name}</span>
					</div>
				{:else if selectedItem}
					<div class="flex items-center justify-between rounded-md border px-3 py-2 text-sm">
						<div class="flex items-center gap-2">
							{#if selectedItem.icon}
								<img src={selectedItem.icon} alt="" class="size-5 object-contain" />
							{/if}
							<span>{selectedItem.name}</span>
						</div>
						<button
							type="button"
							class="text-muted-foreground hover:text-foreground text-xs"
							onclick={() => (selectedItem = null)}
						>
							Change
						</button>
					</div>
				{:else}
					<ItemPicker {items} onselect={onItemSelect} />
				{/if}
			</div>

			<!-- Type toggle -->
			<div class="grid gap-2">
				<Label>Type</Label>
				<div class="flex gap-2">
					<Button
						type="button"
						size="sm"
						variant={tradeType === 'buy' ? 'default' : 'outline'}
						class="flex-1"
						onclick={() => (tradeType = 'buy')}
					>
						Buy
					</Button>
					<Button
						type="button"
						size="sm"
						variant={tradeType === 'sell' ? 'default' : 'outline'}
						class="flex-1"
						onclick={() => (tradeType = 'sell')}
					>
						Sell
					</Button>
				</div>
			</div>

			<!-- Quantity -->
			<div class="grid gap-2">
				<Label>Quantity</Label>
				<Input type="text" inputmode="decimal" placeholder="e.g. 10k" bind:value={quantity} />
			</div>

			<!-- Price -->
			<div class="grid gap-2">
				<Label>Price per unit (gp)</Label>
				<Input type="text" inputmode="decimal" placeholder="e.g. 3.5m" bind:value={pricePerUnit} />
				{#if tradeType === 'sell' && tax > 0}
					<p class="text-xs text-muted-foreground">
						Tax: {tax.toLocaleString()} gp &middot; Net: {netPerUnit.toLocaleString()} gp
					</p>
				{/if}
				{#if qty > 0 && price > 0}
					<p class="text-xs text-muted-foreground">
						Total: {totalCost.toLocaleString()} gp
					</p>
				{/if}
			</div>

			<!-- Notes -->
			<div class="grid gap-2">
				<Label>Notes <span class="text-muted-foreground font-normal">(optional)</span></Label>
				<Input type="text" placeholder="e.g. flipping at GE" bind:value={notes} />
			</div>

			<Dialog.Footer>
				<Button type="submit" disabled={!valid}>
					{isEditing ? 'Save' : 'Log Trade'}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
