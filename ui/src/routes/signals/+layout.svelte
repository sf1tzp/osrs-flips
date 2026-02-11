<script lang="ts">
  import { page } from '$app/state';

  let { children, data } = $props();

  const tabs = [
    { href: '/signals', label: 'Flips' },
    { href: '/signals/momentum', label: 'Momentum' },
    { href: '/signals/compounded', label: 'Compounded' }
  ];

  let compoundedCount = $derived(data.compoundedItems?.length ?? 0);
</script>

<div class="mx-auto max-w-7xl p-4 sm:p-6">
  <div class="mb-4 flex gap-1 border-b">
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
  </div>
  {@render children()}
</div>
