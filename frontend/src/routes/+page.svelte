<script lang="ts">
  import { onMount } from 'svelte';
  import BrowseRow from '$lib/browse/browse-row.svelte';
  import ServiceStrip from '$lib/browse/service-strip.svelte';
  import type { BrowseListMeta, BrowseService } from '$lib/browse/api';
  import {
    discoverListCacheKey,
    discoverServiceListsCacheKey,
    isDiscoverFresh,
    loadBrowseGlobalListsCached,
    loadBrowseListCached,
    loadBrowseServiceListsCached,
    loadBrowseServicesCached,
  } from '$lib/data/browse-cache';
  import { getCached } from '$lib/data/list-cache';
  import { prefetchTabLists } from '$lib/data/prefetch-tabs';
  import MediaActionHost, { type MediaRow } from '$lib/media/media-action-host.svelte';
  import { preloadTabRoutes } from '$lib/navigation/preload-routes';
  import { ApiError } from '$lib/api/client';
  import type { BrowsePage } from '$lib/browse/api';
  import type { SearchResult } from '$lib/types/search';

  const SERVICES_KEY = 'discover:services';
  const GLOBAL_LISTS_KEY = 'discover:global-lists';

  type RowState = {
    loading: boolean;
    error: string;
    results: SearchResult[];
  };

  let services = $state<BrowseService[]>([]);
  let selectedServiceId = $state('');
  let serviceLists = $state<BrowseListMeta[]>([]);
  let globalLists = $state<BrowseListMeta[]>([]);
  let rowState = $state<Record<string, RowState>>({});

  let servicesLoading = $state(true);
  let listsLoading = $state(false);
  let pageError = $state('');
  let statusMessage = $state('');
  let error = $state('');

  let mediaActions = $state<MediaActionHost | undefined>();

  const selectedServiceName = $derived(
    services.find((s) => s.id === selectedServiceId)?.name ?? '',
  );

  /** List kind after the colon (e.g. netflix:drama-series → drama-series). */
  function listKind(id: string): string {
    const i = id.indexOf(':');
    return i >= 0 ? id.slice(i + 1) : id;
  }

  function isSeriesList(id: string): boolean {
    const kind = listKind(id);
    return kind === 'series' || kind.endsWith('-series') || kind === 'trending-series';
  }

  onMount(() => {
    preloadTabRoutes();
    prefetchTabLists();
    initPage();
  });

  function applyRowFromCache(listId: string): boolean {
    const cached = getCached<BrowsePage>(discoverListCacheKey(listId, 1));
    if (!cached) return false;
    rowState = {
      ...rowState,
      [listId]: { loading: false, error: '', results: cached.results },
    };
    return true;
  }

  async function loadList(id: string, options?: { force?: boolean }) {
    const key = discoverListCacheKey(id, 1);
    const cached = getCached<BrowsePage>(key);

    if (cached && !options?.force) {
      rowState = {
        ...rowState,
        [id]: { loading: false, error: '', results: cached.results },
      };
    } else {
      rowState = { ...rowState, [id]: { ...rowState[id], loading: true, error: '' } };
    }

    const needsFetch = options?.force || !cached || !isDiscoverFresh(key);
    if (!needsFetch) return;

    try {
      const page = await loadBrowseListCached(id, 1, options);
      rowState = { ...rowState, [id]: { loading: false, error: '', results: page.results } };
    } catch (e) {
      rowState = {
        ...rowState,
        [id]: {
          loading: false,
          error: e instanceof ApiError ? e.message : 'Failed to load',
          results: rowState[id]?.results ?? [],
        },
      };
    }
  }

  async function initPage() {
    servicesLoading = true;
    pageError = '';

    const cachedServices = getCached<BrowseService[]>(SERVICES_KEY);
    const cachedGlobal = getCached<BrowseListMeta[]>(GLOBAL_LISTS_KEY);
    if (cachedServices) services = cachedServices;
    if (cachedGlobal) {
      globalLists = cachedGlobal;
      for (const list of cachedGlobal) {
        rowState = {
          ...rowState,
          [list.id]: rowState[list.id] ?? { loading: true, error: '', results: [] },
        };
        applyRowFromCache(list.id);
      }
    }

    try {
      const needsMeta =
        !cachedServices ||
        !isDiscoverFresh(SERVICES_KEY) ||
        !cachedGlobal ||
        !isDiscoverFresh(GLOBAL_LISTS_KEY);

      if (needsMeta) {
        const [svc, global] = await Promise.all([
          loadBrowseServicesCached(),
          loadBrowseGlobalListsCached(),
        ]);
        services = svc;
        globalLists = global;
        rowState = Object.fromEntries(
          global.map((l) => [
            l.id,
            rowState[l.id] ?? { loading: true, error: '', results: [] },
          ]),
        );
      }

      if (services.length > 0) {
        await selectService(selectedServiceId || services[0].id);
      }

      await Promise.all(globalLists.map((l) => loadList(l.id)));
    } catch (e) {
      pageError = e instanceof ApiError ? e.message : 'Failed to load discover';
    } finally {
      servicesLoading = false;
    }
  }

  async function selectService(id: string) {
    if (selectedServiceId === id && serviceLists.length > 0) {
      const allCached = serviceLists.every((l) => applyRowFromCache(l.id));
      if (allCached) return;
    }

    selectedServiceId = id;
    listsLoading = true;
    pageError = '';

    const listsKey = discoverServiceListsCacheKey(id);
    const cachedLists = getCached<BrowseListMeta[]>(listsKey);
    if (cachedLists) {
      serviceLists = cachedLists;
      for (const list of cachedLists) {
        rowState = {
          ...rowState,
          [list.id]: rowState[list.id] ?? { loading: true, error: '', results: [] },
        };
        applyRowFromCache(list.id);
      }
    }

    try {
      const needsLists = !cachedLists || !isDiscoverFresh(listsKey);
      if (needsLists) {
        serviceLists = await loadBrowseServiceListsCached(id);
        for (const list of serviceLists) {
          rowState = {
            ...rowState,
            [list.id]: rowState[list.id] ?? { loading: true, error: '', results: [] },
          };
        }
      }

      await Promise.all(serviceLists.map((l) => loadList(l.id)));
    } catch (e) {
      pageError = e instanceof ApiError ? e.message : 'Failed to load service';
      if (!cachedLists) serviceLists = [];
    } finally {
      listsLoading = false;
    }
  }

  function onSearch(row: MediaRow) {
    mediaActions?.runIndexerSearch(row);
  }

  function onDownload(row: MediaRow) {
    mediaActions?.runDownload(row);
  }
</script>

<div class="pb-4">
  <h1 class="mb-1 text-lg font-semibold tracking-tight">Discover</h1>
  <p class="mb-4 text-xs text-muted-foreground">
    Pick a streaming service to browse what’s popular on it.
  </p>

  {#if statusMessage}
    <p class="mb-3 rounded-lg border border-green-500/40 bg-green-500/10 px-3 py-2 text-xs text-green-300">
      {statusMessage}
    </p>
  {/if}

  {#if error}
    <p class="mb-3 text-sm text-red-400">{error}</p>
  {/if}

  {#if pageError}
    <p class="mb-3 text-sm text-red-400">{pageError}</p>
  {/if}

  {#if servicesLoading && services.length === 0}
    <p class="text-sm text-muted-foreground">Loading services…</p>
  {:else if services.length > 0}
    <ServiceStrip {services} selectedId={selectedServiceId} onSelect={selectService} />

    {#if listsLoading && serviceLists.length === 0}
      <p class="mb-4 text-sm text-muted-foreground">
        Loading {services.find((s) => s.id === selectedServiceId)?.name ?? 'service'}…
      </p>
    {:else}
      {#each serviceLists as list, i (list.id)}
        {#if i === 0 && !isSeriesList(list.id)}
          <h2 class="mb-2 text-xs font-semibold uppercase tracking-wide text-white/45">Movies</h2>
        {:else if isSeriesList(list.id) && !isSeriesList(serviceLists[i - 1]?.id ?? '')}
          <h2 class="mb-2 mt-1 text-xs font-semibold uppercase tracking-wide text-white/45">
            Series
          </h2>
        {/if}
        {@const state = rowState[list.id]}
        <BrowseRow
          title="{list.title}{selectedServiceName ? ` on ${selectedServiceName}` : ''}"
          loading={state?.loading}
          error={state?.error}
          results={state?.results ?? []}
          {onSearch}
          {onDownload}
        />
      {/each}
    {/if}

    {#if globalLists.length > 0}
      <div class="mt-2 border-t border-border/30 pt-4">
        {#each globalLists as list, i (list.id)}
          {#if i === 0 && !isSeriesList(list.id)}
            <h2 class="mb-2 text-xs font-semibold uppercase tracking-wide text-white/45">Movies</h2>
          {:else if isSeriesList(list.id) && !isSeriesList(globalLists[i - 1]?.id ?? '')}
            <h2 class="mb-2 mt-1 text-xs font-semibold uppercase tracking-wide text-white/45">
              Series
            </h2>
          {/if}
          {@const state = rowState[list.id]}
          <BrowseRow
            title={list.title}
            loading={state?.loading}
            error={state?.error}
            results={state?.results ?? []}
            {onSearch}
            {onDownload}
          />
        {/each}
      </div>
    {/if}
  {:else if !servicesLoading}
    <p class="text-sm text-muted-foreground">No streaming services available.</p>
  {/if}
</div>

<MediaActionHost bind:this={mediaActions} bind:statusMessage bind:error />
