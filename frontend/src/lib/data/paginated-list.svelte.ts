/**
 * Shared controller for the cache-backed, infinite-scroll list pages
 * (library and requests). Owns the item list, paging, loading/error state,
 * and the stale-while-revalidate reload + load-more flows with race guards.
 *
 * Page-specific behavior is supplied via the config callbacks; the closures
 * are read at call time, so they see the page's current view/query state.
 *
 * Usage (in a component's top-level script, so the runes bind to its scope):
 *   const list = createPaginatedList<Res, Item>({ ... });
 *   // template: list.items, list.loading, list.hasMore, list.reload(), …
 */
export type PaginatedListConfig<TRes, TItem> = {
  /** Cached page-one response, if any (used for instant paint). */
  getCached: () => TRes | undefined;
  /** Whether the cached page-one is fresh enough to skip a refetch. */
  isFresh: () => boolean;
  /** Fetch (and cache) page one. */
  loadPageOne: (options?: { force?: boolean }) => Promise<TRes>;
  /** Fetch a subsequent page (page >= 2) for load-more. */
  fetchMore: (page: number) => Promise<TRes>;
  /** Map a response to its list items. */
  toItems: (res: TRes) => TItem[];
  /** Extract paging metadata from a response. */
  meta: (res: TRes) => { totalPages: number; totalCount: number };
  /** Side effects after page one is applied (poster preload, extra totals). */
  onApply?: (res: TRes, items: TItem[]) => void;
  /** Runs synchronously at the start of every reload (e.g. clear a banner). */
  onReloadStart?: () => void;
  /** Build the user-facing error message for a failed load / load-more. */
  errorMessage: (err: unknown, kind: 'load' | 'more') => string;
};

export function createPaginatedList<TRes, TItem>(config: PaginatedListConfig<TRes, TItem>) {
  let items = $state<TItem[]>([]);
  let loading = $state(false);
  let loadingMore = $state(false);
  let error = $state('');
  let currentPage = $state(1);
  let totalPages = $state(1);
  let totalCount = $state(0);
  let fetchGeneration = 0;

  const hasMore = $derived(items.length > 0 && currentPage < totalPages);

  function applyPageOne(res: TRes) {
    items = config.toItems(res);
    const m = config.meta(res);
    totalPages = Math.max(1, m.totalPages);
    totalCount = m.totalCount;
    currentPage = 1;
    config.onApply?.(res, items);
  }

  async function reload(options?: { force?: boolean }) {
    const generation = ++fetchGeneration;
    loadingMore = false;
    error = '';
    config.onReloadStart?.();
    currentPage = 1;

    const cached = config.getCached();
    if (cached && !options?.force) {
      applyPageOne(cached);
      loading = false;
    } else {
      loading = true;
    }

    const needsFetch = options?.force || !cached || !config.isFresh();
    if (!needsFetch) return;

    try {
      const res = await config.loadPageOne({ force: options?.force });
      if (generation !== fetchGeneration) return;
      applyPageOne(res);
    } catch (err) {
      if (generation !== fetchGeneration) return;
      if (!cached) {
        items = [];
        totalCount = 0;
        error = config.errorMessage(err, 'load');
      }
    } finally {
      if (generation === fetchGeneration) {
        loading = false;
      }
    }
  }

  async function loadMore() {
    if (loading || loadingMore || !hasMore) return;

    const generation = fetchGeneration;
    const nextPage = currentPage + 1;
    loadingMore = true;

    try {
      const res = await config.fetchMore(nextPage);
      if (generation !== fetchGeneration) return;

      items = [...items, ...config.toItems(res)];
      const m = config.meta(res);
      totalPages = Math.max(1, m.totalPages);
      totalCount = m.totalCount;
      currentPage = nextPage;
    } catch (err) {
      if (generation !== fetchGeneration) return;
      error = config.errorMessage(err, 'more');
    } finally {
      if (generation === fetchGeneration) {
        loadingMore = false;
      }
    }
  }

  return {
    get items() {
      return items;
    },
    get loading() {
      return loading;
    },
    get loadingMore() {
      return loadingMore;
    },
    get error() {
      return error;
    },
    set error(value: string) {
      error = value;
    },
    get totalCount() {
      return totalCount;
    },
    get hasMore() {
      return hasMore;
    },
    reload,
    loadMore,
  };
}
