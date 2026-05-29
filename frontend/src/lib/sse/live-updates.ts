import { writable } from 'svelte/store';

/** Bumped when media list data may have changed (SSE or local action). */
export const mediaListVersion = writable(0);

/** Bumped when requests list data may have changed (SSE or local action). */
export const requestsListVersion = writable(0);

export function bumpMediaListVersion(): void {
  mediaListVersion.update((n) => n + 1);
}

export function bumpRequestsListVersion(): void {
  requestsListVersion.update((n) => n + 1);
}
