<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';

	let { data } = $props();
	let item = $derived(data.item);
</script>

<div class="mx-auto max-w-4xl p-4 sm:p-6">
	<div class="mb-6">
		<Button variant="ghost" href="/" class="mb-4 gap-1.5 px-2">
			<ArrowLeft class="size-4" />
			Back to dashboard
		</Button>
		<div class="flex items-center gap-3">
			{#if item.icon}
				<img src={item.icon} alt={item.name} class="size-10 object-contain" />
			{/if}
			<h1 class="text-2xl font-bold">{item.name}</h1>
			{#if item.members}
				<Badge variant="outline">P2P</Badge>
			{/if}
		</div>
		{#if item.examine}
			<p class="text-muted-foreground mt-1">{item.examine}</p>
		{/if}
	</div>

	<div class="text-muted-foreground rounded-lg border p-8 text-center">
		Price history charts coming soon.
	</div>
</div>
