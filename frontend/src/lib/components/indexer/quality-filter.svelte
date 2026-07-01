<script lang="ts">
  import { Button } from '$lib/components/ui/button';
  import { Filter } from 'lucide-svelte';

  interface Props {
    qualities: string[];
    /** Currently selected quality, or null for "all". Bindable. */
    selected: string | null;
    /** When true, show a "Show all" button to clear the selection. */
    clearable?: boolean;
  }

  let { qualities, selected = $bindable(), clearable = false }: Props = $props();

  let open = $state(false);
</script>

<div class="mt-3">
  <Button
    onclick={() => (open = !open)}
    size="sm"
    variant="outline"
    class="h-8 w-full text-xs"
  >
    <Filter class="mr-1 h-3 w-3" />
    {selected ?? 'Filter by quality'}
  </Button>
  {#if open}
    <div class="mt-2 grid grid-cols-2 gap-1.5">
      {#each qualities as quality}
        <Button
          onclick={() => {
            selected = quality;
            open = false;
          }}
          size="sm"
          variant={selected === quality ? 'default' : 'outline'}
          class="h-8 text-xs"
        >
          {quality}
        </Button>
      {/each}
      {#if clearable && selected}
        <Button
          onclick={() => {
            selected = null;
            open = false;
          }}
          size="sm"
          variant="ghost"
          class="col-span-2 h-8 text-xs"
        >
          Show all
        </Button>
      {/if}
    </div>
  {/if}
</div>
