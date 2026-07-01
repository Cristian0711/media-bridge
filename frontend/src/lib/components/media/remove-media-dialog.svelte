<script lang="ts">
  import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogDescription,
    DialogFooter,
  } from '$lib/components/ui/dialog';
  import { Button } from '$lib/components/ui/button';
  import { Trash2 } from 'lucide-svelte';
  import type { MediaLibraryItem } from '$lib/types/media-library';

  interface Props {
    open: boolean;
    item: MediaLibraryItem | null;
    removing?: boolean;
    onOpenChange: (open: boolean) => void;
    onConfirm: () => void;
  }

  let { open, item, removing = false, onOpenChange, onConfirm }: Props = $props();
</script>

<Dialog {open} {onOpenChange}>
  <DialogContent class="max-w-md p-5">
    <DialogHeader class="text-center">
      <DialogTitle>Confirm deletion</DialogTitle>
      <DialogDescription>This action cannot be undone</DialogDescription>
    </DialogHeader>

    {#if item}
      <div class="py-4 text-center">
        <p class="text-sm text-muted-foreground">
          Are you sure you want to remove
          <span class="font-semibold text-foreground">"{item.title}"</span>
          from your library?
        </p>
        {#if item.type !== 'movie'}
          <p class="mt-2 text-xs text-muted-foreground">
            {#if item.episode != null}
              This will remove Season {item.season}, Episode {item.episode}.
            {:else if item.season != null}
              This will remove Season {item.season}.
            {:else}
              This will remove the entire series.
            {/if}
          </p>
        {/if}
      </div>
    {/if}

    <DialogFooter class="flex-row">
      <Button
        variant="outline"
        class="flex-1"
        disabled={removing}
        onclick={() => onOpenChange(false)}
      >
        Cancel
      </Button>
      <Button variant="destructive" class="flex-1" disabled={removing} onclick={onConfirm}>
        <Trash2 class="mr-2 h-4 w-4" />
        {removing ? 'Removing…' : 'Delete'}
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
