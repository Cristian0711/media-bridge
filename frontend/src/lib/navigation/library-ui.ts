import { writable } from 'svelte/store';

export type LibraryView = 'yours' | 'all';

export const libraryView = writable<LibraryView>('yours');

export function setLibraryView(view: LibraryView) {
  libraryView.set(view);
}
