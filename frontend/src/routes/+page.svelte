<script lang="ts">
  import { onMount } from 'svelte';
  import BrowseListSection from '$lib/browse/browse-list-section.svelte';
  import BrowseRowSkeleton from '$lib/browse/browse-row-skeleton.svelte';
  import ServiceStrip from '$lib/browse/service-strip.svelte';
  import { Skeleton } from '$lib/components/ui/skeleton';
  import type { BrowseCatalog, BrowseListMeta, BrowseService } from '$lib/browse/api';
  import {
    applyBrowseCatalogToRowState,
    discoverServiceCatalogCacheKey,
    isDiscoverFresh,
    listsFromCatalog,
    loadBrowseGlobalCatalogCached,
    loadBrowseServiceCatalogCached,
    loadBrowseServicesCached,
  } from '$lib/data/browse-cache';
  import { getCached } from '$lib/data/list-cache';
  import { prefetchTabLists } from '$lib/data/prefetch-tabs';
  import MediaActionHost, { type MediaRow } from '$lib/media/media-action-host.svelte';
  import { preloadTabRoutes } from '$lib/navigation/preload-routes';
  import { ApiError } from '$lib/api/client';
  import type { SearchResult } from '$lib/types/search';

  const SERVICES_KEY = 'discover:services';
  const GLOBAL_CATALOG_KEY = 'discover:global-catalog';

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

  onMount(() => {
    preloadTabRoutes();
    prefetchTabLists();
    initPage();
  });

  function mergeRowState(fromCatalog: Record<string, RowState>) {
    rowState = { ...rowState, ...fromCatalog };
  }

  async function initPage() {
    servicesLoading = true;
    pageError = '';

    const cachedServices = getCached<BrowseService[]>(SERVICES_KEY);
    const cachedGlobalCatalog = getCached<BrowseCatalog>(GLOBAL_CATALOG_KEY);
    if (cachedServices) services = cachedServices;
    if (cachedGlobalCatalog) {
      globalLists = listsFromCatalog(cachedGlobalCatalog);
      mergeRowState(applyBrowseCatalogToRowState(cachedGlobalCatalog));
    }

    try {
      const needsServices = !cachedServices || !isDiscoverFresh(SERVICES_KEY);
      const needsGlobal = !cachedGlobalCatalog || !isDiscoverFresh(GLOBAL_CATALOG_KEY);

      if (needsServices) {
        services = await loadBrowseServicesCached();
      }
      if (needsGlobal) {
        const globalCatalog = await loadBrowseGlobalCatalogCached();
        globalLists = listsFromCatalog(globalCatalog);
        mergeRowState(applyBrowseCatalogToRowState(globalCatalog));
      }

      if (services.length > 0) {
        await selectService(selectedServiceId || services[0].id);
      }
    } catch (e) {
      pageError = e instanceof ApiError ? e.message : 'Failed to load discover';
    } finally {
      servicesLoading = false;
    }
  }

  async function selectService(id: string) {
    const catalogKey = discoverServiceCatalogCacheKey(id);
    const cachedCatalog = getCached<BrowseCatalog>(catalogKey);

    if (selectedServiceId === id && serviceLists.length > 0 && cachedCatalog) {
      return;
    }

    selectedServiceId = id;
    listsLoading = true;
    pageError = '';

    if (cachedCatalog) {
      serviceLists = listsFromCatalog(cachedCatalog);
      mergeRowState(applyBrowseCatalogToRowState(cachedCatalog));
    }

    try {
      const needsCatalog = !cachedCatalog || !isDiscoverFresh(catalogKey);
      if (needsCatalog) {
        const catalog = await loadBrowseServiceCatalogCached(id);
        serviceLists = listsFromCatalog(catalog);
        mergeRowState(applyBrowseCatalogToRowState(catalog));
      }
    } catch (e) {
      pageError = e instanceof ApiError ? e.message : 'Failed to load service';
      if (!cachedCatalog) serviceLists = [];
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
    <div class="mb-4 flex gap-2 overflow-hidden">
      {#each Array(5) as _, i (i)}
        <Skeleton class="h-9 w-24 shrink-0 rounded-full" />
      {/each}
    </div>
    {#each Array(2) as _, i (i)}
      <section class="mb-6">
        <Skeleton class="mb-2 ml-1 h-4 w-40" />
        <BrowseRowSkeleton />
      </section>
    {/each}
  {:else if services.length > 0}
    <ServiceStrip {services} selectedId={selectedServiceId} onSelect={selectService} />

    {#if listsLoading && serviceLists.length === 0}
      {#each Array(2) as _, i (i)}
        <section class="mb-6">
          <Skeleton class="mb-2 ml-1 h-4 w-40" />
          <BrowseRowSkeleton />
        </section>
      {/each}
    {:else}
      <BrowseListSection
        lists={serviceLists}
        {rowState}
        titleSuffix={selectedServiceName ? ` on ${selectedServiceName}` : ''}
        {onSearch}
        {onDownload}
      />
    {/if}

    {#if globalLists.length > 0}
      <div class="mt-2 border-t border-border/30 pt-4">
        <BrowseListSection lists={globalLists} {rowState} {onSearch} {onDownload} />
      </div>
    {/if}
  {:else if !servicesLoading}
    <p class="text-sm text-muted-foreground">No streaming services available.</p>
  {/if}
</div>

<MediaActionHost bind:this={mediaActions} bind:statusMessage bind:error />
