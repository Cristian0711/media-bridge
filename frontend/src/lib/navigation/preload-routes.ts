import { preloadCode } from '$app/navigation';

/** Tab routes to warm after landing on home (excludes `/`). */
export const PRELOAD_TAB_ROUTES = ['/search', '/library', '/requests', '/settings'] as const;

let preloaded = false;

/** Preload route JS chunks once per session (safe to call multiple times). */
export function preloadTabRoutes(): void {
  if (preloaded || typeof window === 'undefined') return;
  preloaded = true;

  const run = () => {
    for (const href of PRELOAD_TAB_ROUTES) {
      void preloadCode(href);
    }
  };

  if ('requestIdleCallback' in window) {
    requestIdleCallback(run, { timeout: 2000 });
  } else {
    setTimeout(run, 0);
  }
}
