<script lang="ts">
  import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogDescription,
  } from '$lib/components/ui/dialog';
  import { Button } from '$lib/components/ui/button';
  import { Tv, CheckCircle2 } from 'lucide-svelte';

  interface Props {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    seasonCount: number;
    showTitle: string;
    onSelectSeason: (season: number | 'all') => void;
    /** Seasons already in the library — marked with a check. */
    ownedSeasons?: Set<number>;
  }

  let {
    open,
    onOpenChange,
    seasonCount,
    showTitle,
    onSelectSeason,
    ownedSeasons = new Set<number>(),
  }: Props = $props();

  const seasons = $derived(Array.from({ length: Math.max(0, seasonCount) }, (_, i) => i + 1));
</script>

<Dialog {open} {onOpenChange}>
  <DialogContent class="max-w-md p-0">
    <div class="shrink-0 border-b border-border/40 px-4 py-3">
      <DialogHeader>
        <DialogTitle>Select Season</DialogTitle>
        <DialogDescription>Choose a season for "{showTitle}"</DialogDescription>
      </DialogHeader>
    </div>

    <div class="min-h-0 overflow-x-hidden overflow-y-auto overscroll-y-contain px-4 py-3">
      <div class="space-y-2">
        <Button
          onclick={() => onSelectSeason('all')}
          variant="default"
          class="w-full justify-start gap-2 text-sm"
        >
          <Tv class="h-4 w-4" />
          Search all seasons
        </Button>

        <div class="grid grid-cols-2 gap-2">
          {#each seasons as season}
            <Button
              onclick={() => onSelectSeason(season)}
              variant="outline"
              class="gap-1.5 text-sm {seasons.length % 2 === 1 && season === seasons.length ? 'col-span-2' : ''}"
            >
              Season {season}
              {#if ownedSeasons.has(season)}
                <CheckCircle2 class="h-3.5 w-3.5 text-green-500" aria-label="In your library" />
              {/if}
            </Button>
          {/each}
        </div>
      </div>
    </div>
  </DialogContent>
</Dialog>
