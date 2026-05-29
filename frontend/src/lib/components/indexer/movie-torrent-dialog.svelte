<script lang="ts">
  import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogDescription,
  } from '$lib/components/ui/dialog';
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { downloadMovie } from '$lib/requests/api';
  import { posterFromItem } from '$lib/search/indexer-params';
  import type { IndexerMovie } from '$lib/types/indexer';
  import type { MediaItem } from '$lib/types/media';
  import { Download, Users, ArrowDownToLine, HardDrive, Filter, Info } from 'lucide-svelte';

  interface Props {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    movies: IndexerMovie[];
    mediaItem: MediaItem | null;
    availableQualities?: string[];
    byIndexer?: Record<string, number>;
    total?: number;
    onQueued?: (message: string) => void;
    onError?: (message: string) => void;
  }

  let {
    open,
    onOpenChange,
    movies,
    mediaItem,
    availableQualities,
    byIndexer,
    total,
    onQueued,
    onError,
  }: Props = $props();

  let selectedQuality = $state<string | null>(null);
  let showQualityFilter = $state(false);
  let showIndexerInfo = $state(false);
  let downloading = $state(false);

  const filteredMovies = $derived(
    selectedQuality !== null && movies
      ? movies.filter((m) => m.quality === selectedQuality)
      : movies || [],
  );

  function formatSize(bytes: number): string {
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
  }

  async function handleDownload(movie: IndexerMovie) {
    if (!mediaItem?.ids.imdb) {
      onError?.('Missing IMDb ID');
      return;
    }
    downloading = true;
    try {
      const ack = await downloadMovie({
        name: mediaItem.title,
        imdb_id: mediaItem.ids.imdb,
        tmdb_id: mediaItem.ids.tmdb?.toString(),
        poster_url: posterFromItem(mediaItem),
        torrent_url: movie.download_link,
        torrent_name: movie.name,
        indexer: movie.indexer_name,
        quality: movie.quality,
      });
      onOpenChange(false);
      onQueued?.(ack.message);
    } catch (e) {
      onOpenChange(false);
      onError?.(e instanceof Error ? e.message : 'Failed to request download');
    } finally {
      downloading = false;
    }
  }
</script>

<Dialog {open} {onOpenChange}>
  <DialogContent class="p-0">
    <div class="shrink-0 border-b border-border/40 px-4 py-3">
      <DialogHeader>
        <div class="flex min-w-0 items-start gap-3">
          <div class="min-w-0 flex-1">
            <DialogTitle>Search Results</DialogTitle>
            <DialogDescription>
              Found {total ?? movies?.length ?? 0} result{(total ?? movies?.length ?? 0) !== 1 ? 's' : ''}
              for "{mediaItem?.title ?? ''}"
            </DialogDescription>
          </div>
          {#if byIndexer && Object.keys(byIndexer).length > 0}
            <Button
              onclick={() => (showIndexerInfo = !showIndexerInfo)}
              size="sm"
              variant="ghost"
              class="h-7 w-7 shrink-0 p-0"
            >
              <Info class="h-4 w-4" />
            </Button>
          {/if}
        </div>
      </DialogHeader>

      {#if showIndexerInfo && byIndexer}
        <div class="mt-3 rounded-lg border border-border/40 bg-muted/30 p-3">
          <div class="mb-2 text-xs font-medium text-muted-foreground">Results by indexer</div>
          {#each Object.entries(byIndexer) as [indexer, count]}
            <div class="flex items-center justify-between py-0.5 text-xs">
              <span>{indexer}</span>
              <Badge variant="secondary">{count}</Badge>
            </div>
          {/each}
        </div>
      {/if}

      {#if availableQualities && availableQualities.length > 1}
        <div class="mt-3">
          <Button
            onclick={() => (showQualityFilter = !showQualityFilter)}
            size="sm"
            variant="outline"
            class="h-8 w-full text-xs"
          >
            <Filter class="mr-1 h-3 w-3" />
            {selectedQuality ?? 'Filter by quality'}
          </Button>
          {#if showQualityFilter}
            <div class="mt-2 grid grid-cols-2 gap-1.5">
              {#each availableQualities as quality}
                <Button
                  onclick={() => {
                    selectedQuality = quality;
                    showQualityFilter = false;
                  }}
                  size="sm"
                  variant={selectedQuality === quality ? 'default' : 'outline'}
                  class="h-8 text-xs"
                >
                  {quality}
                </Button>
              {/each}
              {#if selectedQuality}
                <Button
                  onclick={() => {
                    selectedQuality = null;
                    showQualityFilter = false;
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
      {/if}
    </div>

    <div class="min-h-0 flex-1 overflow-x-hidden overflow-y-auto overscroll-y-contain px-4 py-3">
      {#if filteredMovies.length > 0}
        <div class="space-y-2">
          {#each filteredMovies as movie (`${movie.indexer_name}:${movie.id}:${movie.download_link}`)}
            <div class="min-w-0 overflow-hidden rounded-lg border border-border/40 bg-card/50 p-2.5">
              <div class="mb-1.5 flex min-w-0 items-start justify-between gap-2">
                <h3 class="min-w-0 flex-1 break-all text-xs font-semibold">{movie.name}</h3>
                <Button
                  onclick={() => handleDownload(movie)}
                  size="sm"
                  variant="ghost"
                  class="h-6 w-6 shrink-0 p-0"
                  disabled={downloading}
                >
                  <Download class="h-3.5 w-3.5" />
                </Button>
              </div>
              <div class="mb-2 flex flex-wrap gap-1.5">
                <Badge variant="outline">{movie.indexer_name}</Badge>
                <Badge variant="secondary">{movie.quality}</Badge>
                <Badge variant="outline">{movie.category}</Badge>
                {#if movie.freeleech === 1}
                  <Badge variant="default" class="bg-green-600">Freeleech</Badge>
                {/if}
              </div>
              <div class="flex flex-wrap gap-3 text-[0.65rem] text-muted-foreground">
                <span class="inline-flex items-center gap-1"><HardDrive class="h-3 w-3" />{formatSize(movie.size)}</span>
                <span class="inline-flex items-center gap-1 text-green-500"><ArrowDownToLine class="h-3 w-3" />{movie.seeders}</span>
                <span class="inline-flex items-center gap-1 text-amber-500"><Users class="h-3 w-3" />{movie.leechers}</span>
              </div>
            </div>
          {/each}
        </div>
      {:else}
        <p class="py-12 text-center text-sm text-muted-foreground">No torrents found</p>
      {/if}
    </div>
  </DialogContent>
</Dialog>
