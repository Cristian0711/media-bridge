import { scrollPageToTop } from '$lib/utils/scroll-page';
import { writable } from 'svelte/store';

export type RequestsView = 'yours' | 'all';

export const requestsView = writable<RequestsView>('yours');

export function setRequestsView(view: RequestsView) {
  requestsView.update((current) => {
    if (current !== view) scrollPageToTop();
    return view;
  });
}
