<script lang="ts">
  import { page } from '$app/state';
  import { pinLayoutBottom } from './pin-layout-bottom';
  import LibraryViewTabs from './library-view-tabs.svelte';
  import RequestsViewTabs from './requests-view-tabs.svelte';
  import SearchBar from './search-bar.svelte';
  import TabBar from './tab-bar.svelte';
  import { searchInputFocused } from './search-ui.svelte';

  const onSearch = $derived(page.url.pathname.startsWith('/search'));
  const onLibrary = $derived(page.url.pathname.startsWith('/library'));
  const onRequests = $derived(page.url.pathname.startsWith('/requests'));
</script>

<nav
  use:pinLayoutBottom={onSearch && $searchInputFocused}
  class="bottom-chrome z-50 flex w-full flex-col items-center gap-3 px-4"
  aria-label="Main navigation"
>
  {#if onLibrary}
    <div class="flex w-full justify-center">
      <LibraryViewTabs />
    </div>
  {/if}

  {#if onRequests}
    <div class="flex w-full justify-center">
      <RequestsViewTabs />
    </div>
  {/if}

  {#if onSearch}
    <div class="flex w-full justify-center">
      <SearchBar />
    </div>
  {/if}

  {#if !onSearch || !$searchInputFocused}
    <div class="bottom-chrome-tabs w-full flex justify-center">
      <TabBar />
    </div>
  {/if}
</nav>
