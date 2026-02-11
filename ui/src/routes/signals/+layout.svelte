<script lang="ts">
  import { page } from '$app/state';
  import { Button } from '$lib/components/ui/button';

  let { children } = $props();

  const tabs = [
    { href: '/signals', label: 'Flips' },
    { href: '/signals/momentum', label: 'Momentum' }
  ];
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
        class="inline-block border-b-2 px-4 py-2 text-sm font-medium transition-colors {active
          ? 'border-primary text-primary'
          : 'border-transparent text-muted-foreground hover:border-muted-foreground/50 hover:text-foreground'}"
      >
        {tab.label}
      </a>
    {/each}
  </div>
  {@render children()}
</div>
