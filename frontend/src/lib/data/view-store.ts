import { scrollPageToTop } from '$lib/utils/scroll-page';
import { writable } from 'svelte/store';

/**
 * Creates a writable view-selector store that scrolls the page to the top
 * whenever the selection changes. Backs the "yours / all" toggles on the
 * library and requests list pages.
 */
export function createViewStore<T extends string>(initial: T) {
  const store = writable<T>(initial);

  function setView(view: T) {
    store.update((current) => {
      if (current !== view) scrollPageToTop();
      return view;
    });
  }

  return { store, setView };
}
