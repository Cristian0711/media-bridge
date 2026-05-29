<script lang="ts">
  import type { BrowseService } from '$lib/browse/api';

  interface Props {
    services: BrowseService[];
    selectedId: string;
    onSelect: (id: string) => void;
  }

  let { services, selectedId, onSelect }: Props = $props();
</script>

<section class="mb-5">
  <h2 class="mb-2 px-1 text-sm font-semibold tracking-tight">Popular on your services</h2>
  <div class="-mx-1 flex gap-2.5 overflow-x-auto px-1 pb-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
    {#each services as svc (svc.id)}
      <button
        type="button"
        onclick={() => onSelect(svc.id)}
        class="relative h-[4.5rem] w-[4.5rem] shrink-0 overflow-hidden rounded-xl border transition-colors {selectedId === svc.id
          ? 'border-primary/60 ring-2 ring-primary/30'
          : 'border-border/40 hover:opacity-90'}"
        aria-pressed={selectedId === svc.id}
        aria-label={svc.name}
        title={svc.name}
      >
        {#if svc.logo_url}
          <img
            src={svc.logo_url}
            alt=""
            class="absolute inset-0 h-full w-full object-cover"
            width="144"
            height="144"
            loading="lazy"
            decoding="async"
          />
        {:else}
          <div class="absolute inset-0 bg-muted"></div>
        {/if}
      </button>
    {/each}
  </div>
</section>
