<script lang="ts">
  import MovieTorrentDialog from '$lib/components/indexer/movie-torrent-dialog.svelte';
  import SeasonSelectionDialog from '$lib/components/indexer/season-selection-dialog.svelte';
  import ShowTorrentDialog from '$lib/components/indexer/show-torrent-dialog.svelte';
  import DownloadQualityDialog from '$lib/components/indexer/download-quality-dialog.svelte';
  import { findBestMovie, findBestShow, searchMovies, searchShows } from '$lib/indexer/api';
  import {
    freeleechMovieQualities,
    freeleechShowQualities,
    hasSeeders,
    hasSeedersShow,
  } from '$lib/indexer/download';
  import { invalidateRequestsListCache } from '$lib/data/requests-list-cache';
  import { bumpRequestsListVersion } from '$lib/sse/live-updates';
  import { downloadMovie, downloadShow } from '$lib/requests/api';
  import { ApiError } from '$lib/api/client';
  import { resolveItemForIndexer } from '$lib/search/api';
  import { availabilityItem, fetchOwnedQualities } from '$lib/media/availability';
  import { indexerParamsFor, posterFromItem } from '$lib/search/indexer-params';
  import { MEDIA_UNAVAILABLE, NO_FREELEECH, showToast } from '$lib/toast/toast.svelte';
  import type { IndexerMovie, IndexerShow } from '$lib/types/indexer';
  import type { MediaItem, MediaType } from '$lib/types/media';

  export type MediaRow = { item: MediaItem; mediaType: MediaType };

  type DownloadContext = {
    item: MediaItem;
    mediaType: MediaType;
    season?: number;
  };

  let {
    statusMessage = $bindable(''),
    error = $bindable(''),
  }: {
    statusMessage?: string;
    error?: string;
  } = $props();

  let selectedRow = $state<MediaRow | null>(null);

  let movieDialogOpen = $state(false);
  let movieResults: IndexerMovie[] = $state([]);
  let movieTotal = $state<number>();
  let movieByIndexer = $state<Record<string, number>>();
  let movieQualities = $state<string[]>();

  let seasonDialogOpen = $state(false);
  let seasonCount = $state(0);
  let pendingDownload = $state(false);

  let showDialogOpen = $state(false);
  let showResults: IndexerShow[] = $state([]);
  let unparsedShows: IndexerShow[] = $state([]);
  let selectedSeason = $state<number | 'all'>('all');
  let showTotal = $state<number>();
  let showByIndexer = $state<Record<string, number>>();
  let showQualities = $state<string[]>();

  let downloadQualityOpen = $state(false);
  let downloadQualities = $state<string[]>([]);
  let downloadQualityLoading = $state(false);
  let downloadContext = $state<DownloadContext | null>(null);

  // Qualities already in the library for the active dialog's title (season-scoped for shows).
  let ownedQualities = $state<string[]>([]);

  async function loadOwnedQualities(item: MediaItem, mediaType: MediaType, season?: number) {
    ownedQualities = [];
    ownedQualities = await fetchOwnedQualities(availabilityItem(item, mediaType, season));
  }

  function openMovieDialog(response: Awaited<ReturnType<typeof searchMovies>>) {
    if (!response.movies?.length) {
      showToast(MEDIA_UNAVAILABLE);
      return;
    }
    movieResults = response.movies;
    movieTotal = response.total;
    movieByIndexer = response.by_indexer;
    movieQualities = response.available_qualities;
    movieDialogOpen = true;
  }

  async function openShowDialog(
    response: Awaited<ReturnType<typeof searchShows>>,
    season: number | 'all',
  ) {
    const parsed = response.shows ?? [];
    const unparsed = response.unparsed ?? [];
    if (parsed.length === 0 && unparsed.length === 0) {
      showToast(MEDIA_UNAVAILABLE);
      return;
    }
    showResults = parsed;
    unparsedShows = unparsed;
    selectedSeason = season;
    showTotal = response.total;
    showByIndexer = response.by_indexer;
    showQualities = response.available_qualities;
    showDialogOpen = true;
  }

  async function probeSeasons(item: MediaItem, mediaType: MediaType): Promise<number> {
    const resolved = await resolveItemForIndexer(item, mediaType);
    const response = await searchShows(indexerParamsFor(resolved));
    const seasons = response.available_seasons ?? [];
    if (seasons.length > 0) return Math.max(...seasons);
    const fromShows = [...(response.shows ?? []), ...(response.unparsed ?? [])]
      .map((s) => s.season)
      .filter((n) => n > 0);
    if (fromShows.length > 0) return Math.max(...fromShows);
    return 0;
  }

  function openDownloadQualityPicker(ctx: DownloadContext, qualities: string[]) {
    if (qualities.length === 0) {
      showToast(MEDIA_UNAVAILABLE);
      return;
    }
    downloadContext = ctx;
    downloadQualities = qualities;
    downloadQualityLoading = false;
    downloadQualityOpen = true;
  }

  async function queueMovieDownload(item: MediaItem, quality: string) {
    const { movie } = await findBestMovie({ ...indexerParamsFor(item), quality });
    const ack = await downloadMovie({
      name: item.title,
      imdb_id: item.ids.imdb!,
      tmdb_id: item.ids.tmdb?.toString(),
      poster_url: posterFromItem(item),
      torrent_url: movie.download_link,
      torrent_name: movie.name,
      indexer: movie.indexer_name,
      quality: movie.quality,
    });
    statusMessage = ack.message;
  }

  async function queueShowDownload(item: MediaItem, quality: string, season: number) {
    const params = { ...indexerParamsFor(item), quality, season };
    const { show } = await findBestShow(params);
    const ack = await downloadShow({
      name: item.title,
      season: show.season,
      imdb_id: item.ids.imdb!,
      tvdb_id: item.ids.tvdb?.toString(),
      poster_url: posterFromItem(item),
      torrent_url: show.download_link,
      torrent_name: show.name,
      indexer: show.indexer_name,
      quality: show.quality,
      ...(show.complete_season ? {} : { episode: show.episode }),
    });
    statusMessage = ack.message;
  }

  async function onDownloadQualitySelected(quality: string) {
    const ctx = downloadContext;
    if (!ctx) return;

    downloadQualityLoading = true;
    error = '';

    try {
      if (ctx.mediaType === 'movies') {
        await queueMovieDownload(ctx.item, quality);
      } else if (ctx.season != null) {
        await queueShowDownload(ctx.item, quality, ctx.season);
      }
      downloadQualityOpen = false;
      downloadContext = null;
      invalidateRequestsListCache();
      bumpRequestsListVersion();
    } catch (e) {
      if (
        e instanceof ApiError &&
        (e.message.includes('freeleech') || e.message.includes('no movies') || e.message.includes('no shows'))
      ) {
        showToast(NO_FREELEECH);
      } else {
        error = e instanceof ApiError ? e.message : 'Download request failed';
      }
    } finally {
      downloadQualityLoading = false;
    }
  }

  export async function runIndexerSearch(row: MediaRow) {
    selectedRow = row;
    error = '';
    statusMessage = '';
    ownedQualities = [];

    try {
      const item = await resolveItemForIndexer(row.item, row.mediaType);
      selectedRow = { ...row, item };

      if (row.mediaType === 'movies') {
        const response = await searchMovies(indexerParamsFor(item));
        openMovieDialog(response);
        void loadOwnedQualities(item, 'movies');
      } else {
        pendingDownload = false;
        seasonCount = await probeSeasons(item, row.mediaType);
        seasonDialogOpen = true;
      }
    } catch (e) {
      if (e instanceof ApiError && (e.status === 404 || e.message.includes('no movies') || e.message.includes('no shows'))) {
        showToast(MEDIA_UNAVAILABLE);
      } else {
        error = e instanceof ApiError ? e.message : 'Indexer search failed';
      }
    }
  }

  export async function runDownload(row: MediaRow) {
    selectedRow = row;
    error = '';
    statusMessage = '';
    ownedQualities = [];

    try {
      const item = await resolveItemForIndexer(row.item, row.mediaType);
      selectedRow = { ...row, item };

      if (row.mediaType === 'movies') {
        const response = await searchMovies(indexerParamsFor(item));
        if (!hasSeeders(response.movies ?? [])) {
          showToast(MEDIA_UNAVAILABLE);
          return;
        }
        openDownloadQualityPicker(
          { item, mediaType: 'movies' },
          freeleechMovieQualities(response.movies ?? []),
        );
        void loadOwnedQualities(item, 'movies');
      } else {
        pendingDownload = true;
        downloadContext = { item, mediaType: 'shows' };
        seasonCount = await probeSeasons(item, row.mediaType);
        if (seasonCount <= 0) {
          pendingDownload = false;
          downloadContext = null;
          showToast(MEDIA_UNAVAILABLE);
          return;
        }
        seasonDialogOpen = true;
      }
    } catch (e) {
      if (e instanceof ApiError && (e.status === 404 || e.message.includes('no movies') || e.message.includes('no shows'))) {
        showToast(MEDIA_UNAVAILABLE);
      } else {
        error = e instanceof ApiError ? e.message : 'Download failed';
      }
    }
  }

  async function handleSeasonSelected(season: number | 'all') {
    seasonDialogOpen = false;
    if (!selectedRow) return;

    let item = selectedRow.item;
    try {
      item = await resolveItemForIndexer(item, selectedRow.mediaType);
    } catch (e) {
      pendingDownload = false;
      downloadContext = null;
      error = e instanceof ApiError ? e.message : 'Missing external IDs';
      return;
    }

    const params = indexerParamsFor(item);
    if (season !== 'all') params.season = season;

    try {
      const response = await searchShows(params);

      if (pendingDownload) {
        pendingDownload = false;
        if (season === 'all') {
          downloadContext = null;
          showToast(MEDIA_UNAVAILABLE);
          return;
        }
        const all = [...(response.shows ?? []), ...(response.unparsed ?? [])];
        if (!hasSeedersShow(all)) {
          downloadContext = null;
          showToast(MEDIA_UNAVAILABLE);
          return;
        }
        openDownloadQualityPicker(
          { item, mediaType: 'shows', season },
          freeleechShowQualities(all),
        );
        void loadOwnedQualities(item, 'shows', season);
        return;
      }

      await openShowDialog(response, season);
      void loadOwnedQualities(item, 'shows', season === 'all' ? undefined : season);
    } catch (e) {
      pendingDownload = false;
      downloadContext = null;
      if (e instanceof ApiError && (e.status === 404 || e.message.includes('no shows'))) {
        showToast(MEDIA_UNAVAILABLE);
      } else {
        error = e instanceof ApiError ? e.message : 'Show search failed';
      }
    }
  }

  function onQueued(message: string) {
    statusMessage = message;
    error = '';
    invalidateRequestsListCache();
    bumpRequestsListVersion();
  }

  function onDialogError(message: string) {
    error = message;
    statusMessage = '';
  }
</script>

<MovieTorrentDialog
  open={movieDialogOpen}
  onOpenChange={(open) => (movieDialogOpen = open)}
  movies={movieResults}
  mediaItem={selectedRow?.item ?? null}
  total={movieTotal}
  byIndexer={movieByIndexer}
  availableQualities={movieQualities}
  {ownedQualities}
  onQueued={onQueued}
  onError={onDialogError}
/>

<SeasonSelectionDialog
  open={seasonDialogOpen}
  onOpenChange={(open) => {
    seasonDialogOpen = open;
    if (!open) {
      pendingDownload = false;
      if (!downloadQualityOpen) downloadContext = null;
    }
  }}
  {seasonCount}
  showTitle={selectedRow?.item.title ?? ''}
  onSelectSeason={handleSeasonSelected}
/>

<ShowTorrentDialog
  open={showDialogOpen}
  onOpenChange={(open) => (showDialogOpen = open)}
  shows={showResults}
  {unparsedShows}
  mediaItem={selectedRow?.item ?? null}
  showTitle={selectedRow?.item.title ?? ''}
  season={selectedSeason}
  total={showTotal}
  byIndexer={showByIndexer}
  availableQualities={showQualities}
  {ownedQualities}
  onQueued={onQueued}
  onError={onDialogError}
/>

<DownloadQualityDialog
  open={downloadQualityOpen}
  onOpenChange={(open) => {
    downloadQualityOpen = open;
    if (!open) {
      downloadContext = null;
      downloadQualityLoading = false;
    }
  }}
  title={downloadContext?.item.title ?? ''}
  qualities={downloadQualities}
  loading={downloadQualityLoading}
  {ownedQualities}
  onSelectQuality={onDownloadQualitySelected}
/>
