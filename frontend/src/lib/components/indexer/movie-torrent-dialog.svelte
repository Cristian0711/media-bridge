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
  import TorrentRow from './torrent-row.svelte';
  import QualityFilter from './quality-filter.svelte';
  import type { IndexerMovie } from '$lib/types/indexer';
  import type { MediaItem } from '$lib/types/media';
  import { Info } from 'lucide-svelte';

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
  let showIndexerInfo = $state(false);
  let downloading = $state(false);

  const filteredMovies = $derived(
    selectedQuality !== null && movies
      ? movies.filter((m) => m.quality === selectedQuality)
      : movies || [],
  );

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
        <QualityFilter qualities={availableQualities} bind:selected={selectedQuality} clearable />
      {/if}
    </div>

    <div class="min-h-0 flex-1 overflow-x-hidden overflow-y-auto overscroll-y-contain px-4 py-3">
      {#if filteredMovies.length > 0}
        <div class="space-y-2">
          {#each filteredMovies as movie (`${movie.indexer_name}:${movie.id}:${movie.download_link}`)}
            <TorrentRow
              name={movie.name}
              indexerName={movie.indexer_name}
              quality={movie.quality}
              category={movie.category}
              freeleech={movie.freeleech}
              size={movie.size}
              seeders={movie.seeders}
              leechers={movie.leechers}
              {downloading}
              onDownload={() => handleDownload(movie)}
            />
          {/each}
        </div>
      {:else}
        <p class="py-12 text-center text-sm text-muted-foreground">No torrents found</p>
      {/if}
    </div>
  </DialogContent>
</Dialog>
