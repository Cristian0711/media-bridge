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
  import { downloadShow } from '$lib/requests/api';
  import { posterFromItem } from '$lib/search/indexer-params';
  import TorrentRow from './torrent-row.svelte';
  import QualityFilter from './quality-filter.svelte';
  import type { IndexerShow } from '$lib/types/indexer';
  import type { MediaItem } from '$lib/types/media';
  import { Filter, Info, CheckCircle2 } from 'lucide-svelte';

  interface Props {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    shows: IndexerShow[];
    unparsedShows: IndexerShow[];
    mediaItem: MediaItem | null;
    showTitle: string;
    season?: number | 'all';
    availableQualities?: string[];
    byIndexer?: Record<string, number>;
    total?: number;
    ownedQualities?: string[];
    onQueued?: (message: string) => void;
    onError?: (message: string) => void;
  }

  let {
    open,
    onOpenChange,
    shows,
    unparsedShows,
    mediaItem,
    showTitle,
    season,
    availableQualities,
    byIndexer,
    total,
    ownedQualities = [],
    onQueued,
    onError,
  }: Props = $props();

  let episodeFilter = $state<number | null>(null);
  let showEpisodeFilter = $state(false);
  let selectedQuality = $state<string | null>(null);
  let showIndexerInfo = $state(false);
  let downloading = $state(false);

  const hasIndividualEpisodes = $derived(shows?.some((s) => !s.complete_season) ?? false);

  const availableEpisodes = $derived(() => {
    const episodes = new Set<number>();
    for (const show of shows ?? []) {
      if (!show.complete_season && show.episode > 0) episodes.add(show.episode);
    }
    return Array.from(episodes).sort((a, b) => a - b);
  });

  const filteredShows = $derived(() => {
    let filtered = shows ?? [];
    if (episodeFilter !== null) {
      filtered = filtered.filter((s) => s.episode === episodeFilter || s.complete_season);
    }
    if (selectedQuality !== null) {
      filtered = filtered.filter((s) => s.quality === selectedQuality);
    }
    return filtered;
  });


  async function handleDownload(show: IndexerShow, unparsed = false) {
    if (!mediaItem) return;
    downloading = true;
    try {
      // Unparsed torrents have season/episode 0; API requires season (non-zero).
      const season = unparsed ? 1 : show.season;
      const body = {
        name: mediaItem.title,
        season,
        imdb_id: mediaItem.ids.imdb,
        tvdb_id: mediaItem.ids.tvdb?.toString(),
        poster_url: posterFromItem(mediaItem),
        torrent_url: show.download_link,
        torrent_name: show.name,
        indexer: show.indexer_name,
        quality: show.quality,
        ...(unparsed ? { episode: 0 } : show.complete_season ? {} : { episode: show.episode }),
      };
      const ack = await downloadShow(body);
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

{#snippet seasonBadge(show: IndexerShow)}
  {#if show.season > 0}
    <Badge variant="outline">
      {#if show.complete_season}
        S{String(show.season).padStart(2, '0')} Complete
      {:else}
        S{String(show.season).padStart(2, '0')}E{String(show.episode).padStart(2, '0')}
      {/if}
    </Badge>
  {/if}
{/snippet}

<Dialog {open} {onOpenChange}>
  <DialogContent class="p-0">
    <div class="shrink-0 border-b border-border/40 px-4 py-3">
      <DialogHeader>
        <div class="flex min-w-0 items-start gap-3">
          <div class="min-w-0 flex-1">
            <DialogTitle>Search Results</DialogTitle>
            <DialogDescription>
              {#if season === 'all'}
                All seasons for "{showTitle}" · {total ?? shows?.length ?? 0} results
              {:else if season}
                Season {season} of "{showTitle}" · {total ?? shows?.length ?? 0} results
              {:else}
                "{showTitle}" · {total ?? shows?.length ?? 0} results
              {/if}
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

      {#if ownedQualities.length > 0}
        <div class="mt-3 flex flex-wrap items-center gap-1.5 rounded-lg border border-green-500/30 bg-green-500/10 px-3 py-2 text-xs text-green-300">
          <CheckCircle2 class="h-3.5 w-3.5 shrink-0" />
          <span>In your library{season && season !== 'all' ? ` (S${String(season).padStart(2, '0')})` : ''}:</span>
          {#each ownedQualities as q}
            <Badge variant="secondary">{q}</Badge>
          {/each}
        </div>
      {/if}

      {#if showIndexerInfo && byIndexer}
        <div class="mt-3 rounded-lg border border-border/40 bg-muted/30 p-3 text-xs">
          {#each Object.entries(byIndexer) as [indexer, count]}
            <div class="flex justify-between py-0.5">
              <span>{indexer}</span>
              <Badge variant="secondary">{count}</Badge>
            </div>
          {/each}
        </div>
      {/if}

      {#if availableQualities && availableQualities.length > 1}
        <QualityFilter qualities={availableQualities} bind:selected={selectedQuality} />
      {/if}

      {#if hasIndividualEpisodes}
        <div class="mt-3">
          <Button
            onclick={() => (showEpisodeFilter = !showEpisodeFilter)}
            size="sm"
            variant="outline"
            class="h-8 w-full text-xs"
          >
            <Filter class="mr-1 h-3 w-3" />
            {episodeFilter !== null ? `Episode ${episodeFilter}` : 'Filter by episode'}
          </Button>
          {#if showEpisodeFilter}
            <div class="mt-2 grid grid-cols-5 gap-1.5 sm:grid-cols-8">
              {#each availableEpisodes() as ep}
                <Button
                  onclick={() => {
                    episodeFilter = ep;
                    showEpisodeFilter = false;
                  }}
                  size="sm"
                  variant={episodeFilter === ep ? 'default' : 'outline'}
                  class="h-8 text-xs"
                >
                  {ep}
                </Button>
              {/each}
            </div>
          {/if}
        </div>
      {/if}
    </div>

    <div class="min-h-0 flex-1 overflow-x-hidden overflow-y-auto overscroll-y-contain px-4 py-3">
      {#if filteredShows().length > 0}
        <div class="space-y-2">
          {#each filteredShows() as show (`${show.indexer_name}:${show.id}:${show.download_link}`)}
            <TorrentRow
              name={show.name}
              indexerName={show.indexer_name}
              quality={show.quality}
              category={show.category}
              freeleech={show.freeleech}
              size={show.size}
              seeders={show.seeders}
              leechers={show.leechers}
              crossSeedCount={show.cross_seed_count}
              {downloading}
              onDownload={() => handleDownload(show)}
            >
              {#snippet extraBadges()}{@render seasonBadge(show)}{/snippet}
            </TorrentRow>
          {/each}
        </div>
      {/if}

      {#if unparsedShows?.length > 0}
        <p class="my-3 text-center text-xs text-muted-foreground">Unparsed results</p>
        <div class="space-y-2">
          {#each unparsedShows as show (`${show.indexer_name}:${show.id}:${show.download_link}`)}
            <TorrentRow
              name={show.name}
              indexerName={show.indexer_name}
              quality={show.quality}
              category={show.category}
              freeleech={show.freeleech}
              size={show.size}
              seeders={show.seeders}
              leechers={show.leechers}
              crossSeedCount={show.cross_seed_count}
              {downloading}
              unparsed
              onDownload={() => handleDownload(show, true)}
            >
              {#snippet extraBadges()}{@render seasonBadge(show)}{/snippet}
            </TorrentRow>
          {/each}
        </div>
      {/if}

      {#if filteredShows().length === 0 && (!unparsedShows || unparsedShows.length === 0)}
        <p class="py-12 text-center text-sm text-muted-foreground">No torrents found</p>
      {/if}
    </div>
  </DialogContent>
</Dialog>
