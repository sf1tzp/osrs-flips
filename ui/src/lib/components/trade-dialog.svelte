<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog';
	import { Label } from '$lib/components/ui/label';
	import { Input } from '$lib/components/ui/input';
	import { Button } from '$lib/components/ui/button';
	import ItemPicker from '$lib/components/item-picker.svelte';
	import { tradeStore } from '$lib/portfolio/trade-store.svelte';
	import type { TradePlan } from '$lib/portfolio/types';
	import { parseNumeric } from '$lib/utils';
	import type { DashboardItem } from '$lib/server/db/queries';

	interface Prefill {
		itemId: number;
		itemName: string;
		itemIcon: string | null;
		instaBuyPrice: number | null;
		instaSellPrice: number | null;
		targetSellPrice?: number | null;
	}

	interface Props {
		open: boolean;
		items: DashboardItem[];
		prefill?: Prefill | null;
		editPlan?: TradePlan | null;
	}

	let { open = $bindable(false), items, prefill = null, editPlan = null }: Props = $props();

	let isEditing = $derived(editPlan != null);

	let selectedItem = $state<{ id: number; name: string; icon: string | null } | null>(null);
	let snapshotInstaBuy = $state<number | null>(null);
	let snapshotInstaSell = $state<number | null>(null);
	let quantity = $state('');
	let buyPrice = $state('');
	let sellPrice = $state('');
	let notes = $state('');

	// Reset form when dialog opens
	$effect(() => {
		if (open) {
			if (editPlan) {
				selectedItem = { id: editPlan.itemId, name: editPlan.itemName, icon: editPlan.itemIcon };
				snapshotInstaBuy = editPlan.snapshotInstaBuy;
				snapshotInstaSell = editPlan.snapshotInstaSell;
				quantity = String(editPlan.quantity);
				buyPrice = String(editPlan.buyPrice);
				sellPrice = editPlan.sellPrice != null ? String(editPlan.sellPrice) : '';
				notes = editPlan.notes;
			} else if (prefill) {
				selectedItem = { id: prefill.itemId, name: prefill.itemName, icon: prefill.itemIcon };
				snapshotInstaBuy = prefill.instaBuyPrice;
				snapshotInstaSell = prefill.instaSellPrice;
				buyPrice = prefill.instaSellPrice != null ? String(prefill.instaSellPrice) : '';
				sellPrice = prefill.targetSellPrice != null ? String(prefill.targetSellPrice) : '';
				quantity = '';
				notes = '';
			} else {
				selectedItem = null;
				snapshotInstaBuy = null;
				snapshotInstaSell = null;
				buyPrice = '';
				sellPrice = '';
				quantity = '';
				notes = '';
			}
		}
	});

	let price = $derived(parseNumeric(buyPrice));
	let targetSell = $derived(parseNumeric(sellPrice));
	let qty = $derived(parseNumeric(quantity));
	let totalCost = $derived(qty * price);

	let valid = $derived(selectedItem != null && qty > 0 && price > 0);

	async function submit() {
		if (!valid || !selectedItem) return;

		const plan: TradePlan = {
			id: editPlan ? editPlan.id : Math.random().toString(36).slice(2) + Date.now().toString(36),
			itemId: selectedItem.id,
			itemName: selectedItem.name,
			itemIcon: selectedItem.icon,
			createdAt: editPlan ? editPlan.createdAt : Date.now(),
			snapshotInstaBuy,
			snapshotInstaSell,
			quantity: qty,
			buyPrice: price,
			status: editPlan ? editPlan.status : 'pending',
			filledAt: editPlan ? editPlan.filledAt : null,
			sellPrice: targetSell > 0 ? targetSell : editPlan ? editPlan.sellPrice : null,
			closedAt: editPlan ? editPlan.closedAt : null,
			notes: notes.trim()
		};

		await tradeStore.addPlan(plan);
		open = false;
	}

	function onItemSelect(item: DashboardItem) {
		selectedItem = { id: item.itemId, name: item.name, icon: item.icon };
		snapshotInstaBuy = item.highPrice;
		snapshotInstaSell = item.lowPrice;
		if (item.lowPrice != null) {
			buyPrice = String(item.lowPrice);
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{isEditing ? 'Edit Flip' : 'New Flip'}</Dialog.Title>
			<Dialog.Description
				>{isEditing
					? 'Update flip details.'
					: 'Place a buy offer to start a flip.'}</Dialog.Description
			>
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
							class="text-xs text-muted-foreground hover:text-foreground"
							onclick={() => (selectedItem = null)}
						>
							Change
						</button>
					</div>
				{:else}
					<ItemPicker {items} onselect={onItemSelect} />
				{/if}
			</div>

			<!-- Market snapshot (read-only) -->
			{#if snapshotInstaBuy != null || snapshotInstaSell != null}
				<div class="rounded-md bg-muted/50 px-3 py-2 text-xs text-muted-foreground">
					Insta-buy: {snapshotInstaBuy != null ? snapshotInstaBuy.toLocaleString() : '—'} gp &middot;
					Insta-sell: {snapshotInstaSell != null ? snapshotInstaSell.toLocaleString() : '—'} gp
				</div>
			{/if}

			<!-- Quantity -->
			<div class="grid gap-2">
				<Label>Quantity</Label>
				<Input type="text" inputmode="decimal" placeholder="e.g. 10k" bind:value={quantity} />
			</div>

			<!-- Buy offer price -->
			<div class="grid gap-2">
				<Label>Buy offer price (gp)</Label>
				<Input type="text" inputmode="decimal" placeholder="e.g. 3.5m" bind:value={buyPrice} />
				{#if qty > 0 && price > 0}
					<p class="text-xs text-muted-foreground">
						Total: {totalCost.toLocaleString()} gp
					</p>
				{/if}
			</div>

			<!-- Target sell price -->
			<div class="grid gap-2">
				<Label
					>Target sell price (gp) <span class="font-normal text-muted-foreground">(optional)</span
					></Label
				>
				<Input type="text" inputmode="decimal" placeholder="e.g. 4.2m" bind:value={sellPrice} />
				{#if qty > 0 && targetSell > 0 && price > 0}
					{@const tax = Math.min(Math.floor(targetSell * 0.02), 5_000_000)}
					{@const profitPerUnit = targetSell - tax - price}
					<p
						class="text-xs {profitPerUnit > 0
							? 'text-green-500'
							: profitPerUnit < 0
								? 'text-red-500'
								: 'text-muted-foreground'}"
					>
						Projected: {(profitPerUnit * qty).toLocaleString()} gp ({profitPerUnit > 0
							? '+'
							: ''}{profitPerUnit.toLocaleString()}/ea)
					</p>
				{/if}
			</div>

			<!-- Notes -->
			<div class="grid gap-2">
				<Label>Notes <span class="font-normal text-muted-foreground">(optional)</span></Label>
				<Input type="text" placeholder="e.g. good margin on this flip" bind:value={notes} />
			</div>

			<Dialog.Footer>
				<Button type="submit" disabled={!valid}>
					{isEditing ? 'Save' : 'Create Flip'}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
