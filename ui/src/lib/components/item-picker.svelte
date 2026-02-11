<script lang="ts">
  import { Input } from '$lib/components/ui/input';
  import type { DashboardItem } from '$lib/server/db/queries';

  interface Props {
    items: DashboardItem[];
    onselect: (item: DashboardItem) => void;
  }

  let { items, onselect }: Props = $props();

  let query = $state('');
  let open = $state(false);

  let filtered = $derived.by(() => {
    if (!query.trim()) return [];
    const q = query.toLowerCase();
    return items.filter((i) => i.name.toLowerCase().includes(q)).slice(0, 50);
  });

  function select(item: DashboardItem) {
    onselect(item);
    query = '';
    open = false;
  }
</script>

<div class="relative">
  <Input
    type="text"
    placeholder="Search items..."
    bind:value={query}
    onfocus={() => (open = true)}
    onblur={() => setTimeout(() => (open = false), 150)}
  />
  {#if open && filtered.length > 0}
    <div
      class="absolute z-50 mt-1 max-h-60 w-full overflow-auto rounded-md border bg-popover shadow-md"
    >
      {#each filtered as item (item.itemId)}
        <button
          type="button"
          class="flex w-full cursor-pointer items-center gap-2 px-3 py-2 text-sm hover:bg-accent"
          onmousedown={() => select(item)}
        >
          {#if item.icon}
            <img src={item.icon} alt="" class="size-5 object-contain" />
          {/if}
          <span>{item.name}</span>
        </button>
      {/each}
    </div>
  {/if}
</div>
