export type InfiniteScrollParams = {
  onLoadMore: () => void;
  /** Extra px before the sentinel enters the viewport to start loading */
  rootMargin?: string;
};

export function infiniteScroll(node: HTMLElement, params: InfiniteScrollParams) {
  let onLoadMore = params.onLoadMore;

  const observer = new IntersectionObserver(
    (entries) => {
      if (entries[0]?.isIntersecting) onLoadMore();
    },
    { root: null, rootMargin: params.rootMargin ?? '240px' },
  );

  observer.observe(node);

  return {
    update(next: InfiniteScrollParams) {
      onLoadMore = next.onLoadMore;
    },
    destroy() {
      observer.disconnect();
    },
  };
}
