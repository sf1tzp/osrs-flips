<script lang="ts">
  import { page } from '$app/state';
  import { Input } from '$lib/components/ui/input';
  import { signalFilters } from '$lib/signal-filters.svelte';
  import Filter from '@lucide/svelte/icons/filter';
  import X from '@lucide/svelte/icons/x';

  let { children, data } = $props();

  const tabs = [
    { href: '/signals', label: 'Flips' },
    { href: '/signals/momentum', label: 'Momentum' },
    { href: '/signals/compounded', label: 'Compounded' }
  ];

  let compoundedCount = $derived(data.compoundedItems?.length ?? 0);
  let filtersOpen = $state(false);
</script>

<div class="mx-auto max-w-7xl p-4 sm:p-6">
  <div class="mb-4 flex items-center gap-1 border-b">
    {#each tabs as tab}
      {@const active =
        tab.href === '/signals'
          ? page.url.pathname === '/signals'
          : page.url.pathname.startsWith(tab.href)}
      <a
        href={tab.href}
        class="inline-flex items-center gap-1.5 border-b-2 px-4 py-2 text-sm font-medium transition-colors {active
          ? 'border-primary text-primary'
          : 'border-transparent text-muted-foreground hover:border-muted-foreground/50 hover:text-foreground'}"
      >
        {tab.label}
        {#if tab.href === '/signals/compounded' && compoundedCount > 0}
          <span
            class="inline-flex size-5 items-center justify-center rounded-full bg-primary/10 text-[10px] font-semibold text-primary"
          >
            {compoundedCount}
          </span>
        {/if}
      </a>
    {/each}
    <button
      type="button"
      class="ml-auto inline-flex items-center gap-1.5 border-b-2 px-3 py-2 text-sm font-medium transition-colors {filtersOpen || signalFilters.active
        ? 'border-primary text-primary'
        : 'border-transparent text-muted-foreground hover:text-foreground'}"
      onclick={() => (filtersOpen = !filtersOpen)}
    >
      <Filter class="size-3.5" />
      Filters
      {#if signalFilters.active}
        <span class="size-1.5 rounded-full bg-primary"></span>
      {/if}
    </button>
  </div>

  {#if filtersOpen}
    <div class="mb-4 flex flex-wrap items-end gap-4 rounded-lg border bg-muted/30 px-4 py-3">
      <div class="grid gap-1">
        <span class="text-[11px] font-medium text-muted-foreground">Buy Price</span>
        <div class="flex gap-1.5">
          <Input
            type="text"
            inputmode="decimal"
            placeholder="Min"
            class="h-7 w-20 text-xs"
            bind:value={signalFilters.buyPriceMin}
          />
          <Input
            type="text"
            inputmode="decimal"
            placeholder="Max"
            class="h-7 w-20 text-xs"
            bind:value={signalFilters.buyPriceMax}
          />
        </div>
      </div>
      <div class="grid gap-1">
        <span class="text-[11px] font-medium text-muted-foreground">Sell Price</span>
        <div class="flex gap-1.5">
          <Input
            type="text"
            inputmode="decimal"
            placeholder="Min"
            class="h-7 w-20 text-xs"
            bind:value={signalFilters.sellPriceMin}
          />
          <Input
            type="text"
            inputmode="decimal"
            placeholder="Max"
            class="h-7 w-20 text-xs"
            bind:value={signalFilters.sellPriceMax}
          />
        </div>
      </div>
      <div class="grid gap-1">
        <span class="text-[11px] font-medium text-muted-foreground">24h Volume</span>
        <div class="flex gap-1.5">
          <Input
            type="text"
            inputmode="decimal"
            placeholder="Min"
            class="h-7 w-20 text-xs"
            bind:value={signalFilters.volumeMin}
          />
          <Input
            type="text"
            inputmode="decimal"
            placeholder="Max"
            class="h-7 w-20 text-xs"
            bind:value={signalFilters.volumeMax}
          />
        </div>
      </div>
      {#if signalFilters.active}
        <button
          type="button"
          class="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
          onclick={() => signalFilters.reset()}
        >
          <X class="size-3" />
          Clear
        </button>
      {/if}
    </div>
  {/if}

  {@render children()}
</div>
