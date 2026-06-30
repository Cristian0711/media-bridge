/// <reference types="@sveltejs/kit" />
/// <reference no-default-lib="true"/>
/// <reference lib="esnext" />
/// <reference lib="webworker" />

// SvelteKit injects the build manifest here. `build` = hashed, immutable app
// assets; `files` = everything in static/; `version` = a per-build id we use to
// scope (and garbage-collect) the cache.
import { build, files, version } from '$service-worker';

const sw = self as unknown as ServiceWorkerGlobalScope;

const CACHE = `media-bridge-${version}`;

// Content-addressed app assets are safe to precache and serve cache-first; a new
// build changes their hashes (and `version`), so they never go stale.
const PRECACHE = [...build, ...files];
const PRECACHE_SET = new Set(PRECACHE);

// Only the browse catalog is global (not per-user) and changes ~daily, so it's
// the one API surface safe to cache here. User-scoped lists (library/requests)
// are intentionally NOT cached in the SW — the in-memory store already makes
// them fast in-session, and caching them risks cross-user leakage on a shared
// device.
const SWR_API = /^\/api\/v1\/browse\//;

sw.addEventListener('install', (event) => {
  event.waitUntil(
    caches
      .open(CACHE)
      .then((cache) => cache.addAll(PRECACHE))
      .then(() => sw.skipWaiting()),
  );
});

sw.addEventListener('activate', (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => sw.clients.claim()),
  );
});

async function cacheFirst(request: Request): Promise<Response> {
  const cache = await caches.open(CACHE);
  const cached = await cache.match(request);
  if (cached) return cached;
  const res = await fetch(request);
  if (res.ok) cache.put(request, res.clone());
  return res;
}

async function staleWhileRevalidate(request: Request): Promise<Response> {
  const cache = await caches.open(CACHE);
  const cached = await cache.match(request);
  const network = fetch(request)
    .then((res) => {
      if (res.ok) cache.put(request, res.clone());
      return res;
    })
    .catch(() => undefined);
  return cached ?? (await network) ?? fetch(request);
}

// Network-first so an online client always gets the current shell (and thus the
// current JS hashes) — this preserves the no-store-on-index.html behavior that
// guards against stale iOS PWA shells. The cached shell is only a fallback when
// the network is unreachable.
async function networkFirstNavigation(request: Request): Promise<Response> {
  const cache = await caches.open(CACHE);
  try {
    const res = await fetch(request);
    if (res.ok) cache.put(request, res.clone());
    return res;
  } catch {
    return (await cache.match(request)) ?? (await cache.match('/')) ?? Response.error();
  }
}

sw.addEventListener('fetch', (event) => {
  const { request } = event;
  if (request.method !== 'GET') return;

  const url = new URL(request.url);

  // Cross-origin (e.g. TMDB posters, Google Fonts) — let the browser HTTP cache
  // handle these; intercepting opaque cross-origin responses adds no value here.
  if (url.origin !== sw.location.origin) return;

  // Immutable, content-hashed build assets.
  if (PRECACHE_SET.has(url.pathname)) {
    event.respondWith(cacheFirst(request));
    return;
  }

  // SSE streams must never be intercepted or cached.
  if (url.pathname.endsWith('/events')) return;

  // Global browse catalog — stale-while-revalidate for an instant cold launch.
  if (SWR_API.test(url.pathname)) {
    event.respondWith(staleWhileRevalidate(request));
    return;
  }

  // SPA navigations — network-first with an offline shell fallback.
  if (request.mode === 'navigate') {
    event.respondWith(networkFirstNavigation(request));
    return;
  }

  // Everything else (other API calls, etc.) goes straight to the network.
});
