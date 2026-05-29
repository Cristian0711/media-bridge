import { writable } from 'svelte/store';

/** True while the search input is focused (keyboard open). Hides the tab bar. */
export const searchInputFocused = writable(false);

export function setSearchInputFocused(focused: boolean) {
  searchInputFocused.set(focused);
}

export function clearSearchInputFocused() {
  searchInputFocused.set(false);
}
