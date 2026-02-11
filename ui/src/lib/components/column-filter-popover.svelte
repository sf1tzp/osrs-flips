<script lang="ts">
	import * as Popover from '$lib/components/ui/popover';
	import { Input } from '$lib/components/ui/input';
	import Filter from '@lucide/svelte/icons/filter';

	let {
		min = $bindable(''),
		max = $bindable(''),
		label
	}: {
		min: string;
		max: string;
		label: string;
	} = $props();

	let active = $derived(min !== '' || max !== '');
</script>

<Popover.Root>
	<Popover.Trigger
		class="inline-flex cursor-pointer items-center"
		onclick={(e: MouseEvent) => e.stopPropagation()}
	>
		<Filter class="size-3 {active ? 'text-primary' : 'opacity-30'}" />
	</Popover.Trigger>
	<Popover.Content class="w-48 p-3" align="start">
		<p class="mb-2 text-xs font-medium text-muted-foreground">{label}</p>
		<div class="flex flex-col gap-2">
			<Input type="text" placeholder="Min" class="h-7 text-xs" bind:value={min} />
			<Input type="text" placeholder="Max" class="h-7 text-xs" bind:value={max} />
		</div>
	</Popover.Content>
</Popover.Root>
