<script lang="ts">
  import { page } from '$app/state';
  import { pinLayoutBottom } from './pin-layout-bottom';
  import ViewTabs from './view-tabs.svelte';
  import SearchBar from './search-bar.svelte';
  import TabBar from './tab-bar.svelte';
  import { searchInputFocused } from './search-ui.svelte';
  import { libraryView, setLibraryView } from '$lib/data/library-view';
  import { requestsView, setRequestsView } from '$lib/data/requests-view';
  import { User, Users } from 'lucide-svelte';

  const onSearch = $derived(page.url.pathname.startsWith('/search'));
  const onLibrary = $derived(page.url.pathname.startsWith('/library'));
  const onRequests = $derived(page.url.pathname.startsWith('/requests'));

  const libraryTabs = [
    { id: 'yours', label: 'Your Media', icon: User },
    { id: 'all', label: 'All Media', icon: Users },
  ];
  const requestsTabs = [
    { id: 'yours', label: 'Your Requests', icon: User },
    { id: 'all', label: 'All Requests', icon: Users },
  ];
</script>

<nav
  use:pinLayoutBottom={(onSearch || onLibrary) && $searchInputFocused}
  class="bottom-chrome z-50 flex w-full flex-col items-center gap-3 px-4"
  aria-label="Main navigation"
>
  {#if onLibrary && !$searchInputFocused}
    <div class="flex w-full justify-center">
      <ViewTabs
        tabs={libraryTabs}
        activeId={$libraryView}
        onSelect={(id) => setLibraryView(id as 'yours' | 'all')}
        ariaLabel="Library view"
      />
    </div>
  {/if}

  {#if onRequests}
    <div class="flex w-full justify-center">
      <ViewTabs
        tabs={requestsTabs}
        activeId={$requestsView}
        onSelect={(id) => setRequestsView(id as 'yours' | 'all')}
        ariaLabel="Requests view"
      />
    </div>
  {/if}

  {#if onSearch}
    <div class="flex w-full justify-center">
      <SearchBar />
    </div>
  {/if}

  {#if (!onSearch && !onLibrary) || !$searchInputFocused}
    <div class="bottom-chrome-tabs w-full flex justify-center">
      <TabBar />
    </div>
  {/if}
</nav>
