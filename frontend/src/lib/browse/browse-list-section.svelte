<script lang="ts">
  import BrowseRow from './browse-row.svelte';
  import { isSeriesList } from './list-kind';
  import type { BrowseListMeta } from './api';
  import type { MediaRow } from '$lib/media/media-action-host.svelte';
  import type { SearchResult } from '$lib/types/search';

  type RowState = { loading: boolean; error: string; results: SearchResult[] };

  interface Props {
    lists: BrowseListMeta[];
    rowState: Record<string, RowState>;
    /** Appended to each row title (e.g. " on Netflix"). */
    titleSuffix?: string;
    onSearch: (row: MediaRow) => void;
    onDownload: (row: MediaRow) => void;
  }

  let { lists, rowState, titleSuffix = '', onSearch, onDownload }: Props = $props();
</script>

{#each lists as list, i (list.id)}
  {#if i === 0 && !isSeriesList(list.id)}
    <h2 class="mb-2 text-xs font-semibold uppercase tracking-wide text-white/45">Movies</h2>
  {:else if isSeriesList(list.id) && !isSeriesList(lists[i - 1]?.id ?? '')}
    <h2 class="mb-2 mt-1 text-xs font-semibold uppercase tracking-wide text-white/45">Series</h2>
  {/if}
  {@const state = rowState[list.id]}
  <BrowseRow
    title="{list.title}{titleSuffix}"
    loading={state?.loading}
    error={state?.error}
    results={state?.results ?? []}
    {onSearch}
    {onDownload}
  />
{/each}
