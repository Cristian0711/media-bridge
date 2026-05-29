<script lang="ts">
  import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogDescription,
  } from '$lib/components/ui/dialog';
  import { Button } from '$lib/components/ui/button';
  import { Download } from 'lucide-svelte';

  interface Props {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    title: string;
    qualities: string[];
    loading?: boolean;
    onSelectQuality: (quality: string) => void;
  }

  let {
    open,
    onOpenChange,
    title,
    qualities,
    loading = false,
    onSelectQuality,
  }: Props = $props();
</script>

<Dialog {open} {onOpenChange}>
  <DialogContent class="max-w-md p-0">
    <div class="shrink-0 border-b border-border/40 px-4 py-3">
      <DialogHeader>
        <DialogTitle>Choose quality</DialogTitle>
        <DialogDescription>
          Select quality to download for "{title}" (freeleech only)
        </DialogDescription>
      </DialogHeader>
    </div>

    <div class="min-h-0 overflow-x-hidden overflow-y-auto overscroll-y-contain px-4 py-3">
      {#if loading}
        <p class="py-6 text-center text-sm text-muted-foreground">Finding torrent…</p>
      {:else}
        <div class="flex flex-col gap-2">
          {#each qualities as quality (quality)}
            <Button
              onclick={() => onSelectQuality(quality)}
              variant="outline"
              class="h-10 w-full justify-start gap-2 px-3 text-sm"
            >
              <Download class="h-3.5 w-3.5 shrink-0" />
              {quality}
            </Button>
          {/each}
        </div>
      {/if}
    </div>
  </DialogContent>
</Dialog>
