# Frontend Perceived-Performance Plan

Goal: make the app **feel faster** without changing the design. Every item below is
behavior- and pixel-preserving — we change *when* and *how* bytes arrive, never the
markup, classes, or layout. Frontend and backend/serving changes are both in scope.

Stack: SvelteKit 2 (adapter-static SPA, `ssr=false` + `prerender`) + Svelte 5 runes +
Tailwind 4 + bits-ui, served by OpenResty (nginx) in front of a Go backend, behind
Cloudflare.

---

## What's already good (don't redo these)

The app is not naive — these levers are already pulled, and the plan builds on them:

- **Stale-while-revalidate cache** with in-flight dedup ([list-cache.ts](../frontend/src/lib/data/list-cache.ts)) — cached pages paint instantly, refetch in the background.
- **Idle prefetch** of every tab's page-one data after landing on home ([prefetch-tabs.ts](../frontend/src/lib/data/prefetch-tabs.ts), [browse-cache.ts](../frontend/src/lib/data/browse-cache.ts)).
- **Route-chunk preloading** on idle ([preload-routes.ts](../frontend/src/lib/navigation/preload-routes.ts)) + `data-sveltekit-preload-data="hover"` ([app.html](../frontend/src/app.html)).
- **Poster preloading** into the browser image cache ([poster-preload.ts](../frontend/src/lib/utils/poster-preload.ts)).
- **Live updates over a single SSE connection** instead of polling ([+layout.svelte](../frontend/src/routes/+layout.svelte)).
- **nginx auth-token cache** (60s shared dict) so most API calls skip the auth round-trip ([auth.lua](../frontend/nginx/lua/auth.lua)).
- **Immutable long-cache** on hashed `_app/` assets ([nginx.conf](../frontend/nginx/nginx.conf)).

So the gains below are the *gaps* that remain, ranked by impact ÷ effort.

---

## Tier 1 — Biggest win, lowest effort (do these first)

### 1.1 nginx serving-layer tuning — see [Phase N](#phase-n--nginx-serving-layer)
The single highest-ROI change (text compression) plus several other serving-layer wins live
in their own phase below, since the user asked for an nginx-specific pass. The core ones
(gzip + precompress, `open_file_cache`, proxy-buffering split) are **already applied** to
[nginx.conf](../nginx/nginx.conf) and [Dockerfile](../nginx/Dockerfile). Jump to
**[Phase N — nginx serving layer](#phase-n--nginx-serving-layer)**.

---

### 1.2 Preconnect to the poster CDN  ✅ applied
Every poster comes from `https://image.tmdb.org` ([tmdb_client.go](../backend/internal/search/tmdb_client.go) builds `…/t/p/w500/…`),
but [app.html](../frontend/src/app.html) only preconnects to Google Fonts. The first poster
on Home/Library therefore eats a cold DNS + TLS handshake to a new origin.

**Change (frontend):** add to `<head>` in [app.html](../frontend/src/app.html):

```html
<link rel="preconnect" href="https://image.tmdb.org" crossorigin />
<link rel="dns-prefetch" href="https://image.tmdb.org" />
```

**Effect:** posters start downloading ~100–300 ms sooner on first view. Near-zero cost.

---

### 1.3 Right-size poster images per use (stop shipping w500 for 44px thumbs)  ✅ applied
Implemented via `posterAtWidth()` + `POSTER_THUMB_WIDTH` (w185) / `POSTER_CARD_WIDTH` (w342)
in [poster-url.ts](../frontend/src/lib/utils/poster-url.ts); the list/card components and the
poster *preloaders* ([poster-preload.ts](../frontend/src/lib/utils/poster-preload.ts)) request
the **same** rendition so preloads stay cache hits. Original notes below.

The backend hands the frontend a single `w500` poster URL. That's fine for the 120 px
Discover cards, but **wildly oversized** for the 44 px list thumbnails in Library/Requests
([poster-thumb.svelte](../frontend/src/lib/components/media/poster-thumb.svelte) renders at
`w-11 h-16` = 44×64). A `w500` JPEG is ~40–60 KB; `w154` is ~8–12 KB — and we load 20+ per
page.

The poster URLs are a known, rewritable CDN pattern (`/t/p/w500/`), so we can pick a size at
the call site without touching the backend or the design.

**Change (frontend):** in [poster-url.ts](../frontend/src/lib/utils/poster-url.ts) add a
size-aware variant:

```ts
// Rewrite a known image.tmdb.org URL to a different rendition width.
export function posterAtWidth(url: string | null | undefined, w: 'w154'|'w185'|'w342'|'w500') {
  const abs = normalizePosterUrl(url);
  return abs?.replace(/\/t\/p\/w\d+\//, `/t/p/${w}/`);
}
```

Then:
- list thumbnails ([poster-thumb.svelte](../frontend/src/lib/components/media/poster-thumb.svelte),
  [media-card.svelte](../frontend/src/lib/components/media/media-card.svelte)) → `w154`/`w185`.
- Discover strip cards ([browse-row.svelte](../frontend/src/lib/browse/browse-row.svelte),
  120 px wide ≈ `w342` at 3× DPR).
- preload using the **same** small URL ([poster-preload.ts](../frontend/src/lib/utils/poster-preload.ts))
  so the cache key matches what the `<img>` requests.

**Effect:** library/requests/search lists drop from ~1 MB to ~200 KB of poster bytes per
page; posters paint noticeably faster. Pixel output is identical (we were downscaling a
huge image anyway).

*(Backend alternative if URL-rewriting feels fragile: have the API emit both a `poster_url`
and a `poster_thumb_url`, or a srcset-friendly base. Frontend rewrite is lower-risk and
ships today.)*

---

## Tier 2 — High impact, moderate effort

### 2.1 Add a service worker (app-shell + API SWR cache)  ✅ applied
The app is installed as a PWA (manifest, iOS standalone handling) but **had no service
worker** — so a repeat/relaunch hit the network for the shell and all data. A SW is the
single biggest lever for *repeat-visit* perceived speed and the only one that survives a cold
app launch.

**Implemented** in [service-worker.ts](../frontend/src/service-worker.ts) (SvelteKit
auto-registers it), deliberately conservative:
- **Precache** the content-hashed app assets (`$service-worker`'s `build` + `files`), served
  **cache-first** — they're immutable, and a new build changes their hashes + `version`, so
  they never go stale. Old version caches are dropped on `activate`.
- **Navigations → network-first**, with the cached shell only as an *offline* fallback. This
  preserves the deliberate `no-store` on `index.html` (the iOS stale-shell guard): an online
  client always pulls the current shell, so it can never be served stale JS hashes.
- **Browse catalog (`/api/v1/browse/`) → stale-while-revalidate** for an instant cold launch.
  This is the *only* API surface cached, because it's global (not per-user). User-scoped
  lists (library/requests) are intentionally left to the network — the in-memory store
  already makes them fast in-session, and SW-caching per-user data risks cross-user leakage
  on a shared device. *(Note: catalogs carry a per-user `available` flag, but the SW cache is
  per-device/per-user, and SWR revalidates, so this is fine here — see §3.4 for why the same
  data must NOT be cached at a shared CDN.)*
- **Never touches** SSE (`*/events`), non-GET requests, or cross-origin (TMDB/fonts).

**Rollback:** if it ever misbehaves, ship a no-op `service-worker.ts` (empty install/activate
that `skipWaiting()` + `clients.claim()` + clears caches) to evict the old one.

**Verify (must run in a real build — couldn't build here on Node 18):** second launch paints
Home with zero network for shell + catalog; works offline for already-seen data; SSE live
updates still arrive; a deploy with new asset hashes loads cleanly (no stale shell).

### 2.2 Skeleton placeholders instead of "Loading…" text
Pages currently show a text line ("Loading services…", "Searching…", "Loading…") while data
loads ([+page.svelte](../frontend/src/routes/+page.svelte), [search/+page.svelte](../frontend/src/routes/search/+page.svelte),
[browse-row.svelte](../frontend/src/lib/browse/browse-row.svelte)). Skeletons that match the
existing card geometry read as "almost there" rather than "blank/stuck," which measurably
improves *perceived* speed even when wall-clock time is unchanged.

**Change (frontend):** add a `<PosterSkeleton>` / row-skeleton using the **exact existing
dimensions** (`h-[19rem] w-[7.5rem]` cards, `h-16 w-11` thumbs) with a subtle Tailwind
`animate-pulse` on `bg-muted`. This stays within the design system — same boxes, same sizes,
just shown before data instead of a text string. Render N skeletons matching the page size
(e.g. 20 for lists, ~6 per Discover row).

> Note: this is the one item that touches markup. It's additive (a loading state), not a
> redesign, and reuses existing tokens/sizes. Flag for design sign-off if "no markup change
> at all" is strict.

### 2.3 Optimistic / instant tab paint from cache (audit the gap)
Library/Requests already paint from cache via `createPaginatedList`
([paginated-list.svelte.ts](../frontend/src/lib/data/paginated-list.svelte.ts)). Confirm the
**search** page and the Discover **service switch** also paint cached data before awaiting —
e.g. switching streaming services in [+page.svelte](../frontend/src/routes/+page.svelte)
already reads `getCached` first; make sure no `await` blanks the list before the cached rows
render. Cheap audit, removes any residual flashes.

---

## Tier 3 — Polish (smaller or situational gains)

### 3.1 Self-host (or preload) the font; trim weights
[app.html](../frontend/src/app.html) loads Inter from Google Fonts as a render-blocking
stylesheet on a third-party origin, pulling **4 weights** (400/500/600/700). Options, in
order of payoff:
- Self-host Inter as `woff2` under `static/` and `@font-face` it — removes a cross-origin
  round-trip and a blocking external CSS request; pairs well with the SW precache.
- Or at minimum drop unused weights and keep `display=swap` (already set).
- Add `<link rel="preload" as="font" crossorigin>` for the primary weight if self-hosted.

### 3.2 Image decode/CLS hints on the grid images  ✅ applied (LCP-priority part deferred)
Added `decoding="async"` + explicit `width`/`height` to the Discover
([browse-row.svelte](../frontend/src/lib/browse/browse-row.svelte)) and search-result
([media-card.svelte](../frontend/src/lib/components/media/media-card.svelte)) images;
[poster-thumb.svelte](../frontend/src/lib/components/media/poster-thumb.svelte) already had
them. **Deferred:** giving the first Discover row `fetchpriority="high"` + `loading="eager"`
for LCP — the row component doesn't know its position; pass an `eager`/`priority` prop from
the page for the first row in a follow-up.

### 3.3 Parallelize the discover warm  ✅ applied
`prefetchDiscover` ([browse-cache.ts](../frontend/src/lib/data/browse-cache.ts)) now awaits
services (needed for their ids), then warms the global catalog **and** all per-service
catalogs together in one `Promise.all`. Shaves one serial round-trip off the background warm.

### 3.4 Backend `Cache-Control` on cacheable GETs
The browse catalog is server-cached 24 h and library/requests pages are stable for seconds.
Emitting `Cache-Control: max-age=…, stale-while-revalidate=…` on those responses lets both
the browser HTTP cache and the new service worker (2.1) revalidate correctly without bespoke
TTL logic, and lets Cloudflare cache the catalog at the edge. (SSE/auth/mutations stay
`no-store`.)

### 3.5 SvelteKit view transitions (optional, design-neutral)
`onNavigate` + the View Transitions API gives a subtle cross-fade between tabs that hides the
last few ms of hydration/paint. Purely additive, no layout change, easy to gate behind
`prefers-reduced-motion`. Skip if even a cross-fade counts as a "design change."

---

## Phase N — nginx serving layer

A focused pass over [nginx.conf](../nginx/nginx.conf) (OpenResty in front of the Go backend,
behind Cloudflare). Goal: cut bytes on the wire and shave latency off both static-asset and
API hits, without changing any response *content*. The static build is ~264 KB JS + ~49 KB
CSS and was served **raw**; API JSON was served raw and **unbuffered**.

### Applied (committed to the repo)

These are done and safe — verify with `nginx -t` / a Lighthouse pass after the next image build.

- **Text compression + precompressed static.** Added `gzip on` (+ `gzip_vary`,
  `gzip_proxied any`, an explicit `gzip_types` list, `gzip_comp_level 5`,
  `gzip_min_length 1024`) and `gzip_static on`. The [Dockerfile](../nginx/Dockerfile) now
  precompresses every `*.js/css/svg/json/webmanifest/html` in `build/` with `gzip -9` at
  image-build time, so hashed/immutable assets are served from a `.gz` on disk instead of
  being recompressed per request. JSON API responses get compressed on the fly.
  - *Effect:* ~70% smaller JS/CSS over the wire (≈313 KB → ~90 KB) and similarly smaller
    catalog/list JSON. Faster first paint and faster every API response.
  - *SSE safety:* `text/event-stream` is intentionally **excluded** from `gzip_types`, so
    live streams are never buffered by the gzip filter.

- **`open_file_cache`** (10k entries, 60s) — caches fd + `stat()` for static assets so
  repeat hits skip the filesystem lookup. Safe because `_app/` assets are hashed/immutable.

- **Proxy-buffering split for `/api/`.** Flipped `proxy_buffering off` → `on` (with
  `proxy_buffers 16 16k`, `proxy_busy_buffers_size 32k`). Buffering frees the backend
  connection as soon as the response is read (less head-of-line blocking from slow clients)
  and lets gzip apply to JSON. **SSE still streams live** because all three SSE handlers
  ([sse/handler.go](../backend/internal/sse/handler.go),
  [requests/stream.go](../backend/internal/requests/stream.go),
  [qbittorrent/handler.go](../backend/internal/qbittorrent/handler.go)) already send
  `X-Accel-Buffering: no`, which nginx honors per-response — so one `/api/` location serves
  both buffered JSON and unbuffered streams correctly. Long `proxy_read/send_timeout`
  retained for idle-between-events streams.

- **`server_tokens off`** (hide version; tiny response shrink + less fingerprinting) and
  **`listen 80 reuseport`** (kernel distributes new connections across workers — smoother
  under burst).

### Recommended next (needs a build-image or config decision)

- **Brotli** (`brotli on; brotli_static on;`) is ~15–20% smaller than gzip for JS/CSS, but
  the stock `openresty/openresty:bullseye` base image **does not bundle `ngx_brotli`** —
  adding it means compiling the dynamic module into the image (or switching to a base that
  ships it) and precompressing `.br` alongside `.gz`. Cloudflare already brotli-compresses to
  the *client*, so this only helps the origin→Cloudflare hop and direct origin hits — do it
  if/when you build a custom OpenResty image, otherwise gzip is sufficient.

- **Real client IP behind Cloudflare** (correctness/observability, not latency). The access
  log records Cloudflare's IP as `$remote_addr`. Add `ngx_http_realip_module` config —
  `set_real_ip_from <Cloudflare ranges>; real_ip_header CF-Connecting-IP;` — so logs/traces
  attribute the true client. Keep the Cloudflare range list updated (or pull dynamically).

- **Cache-Control on cacheable API GETs** (pairs with frontend §3.4 / the service worker).
  The browse catalog is server-cached 24h; emitting `Cache-Control: public,
  max-age=…, stale-while-revalidate=…` from the backend lets nginx (and Cloudflare, and the
  SW) revalidate it at the edge instead of always hitting the origin. This is a backend
  header change; nginx just forwards it. Keep `no-store` on SSE/auth/mutations.

### Deliberately NOT changed (and why)

- **HTTP/2 on `listen`** — left as HTTP/1.1. TLS + HTTP/2/3 to the client terminate at
  Cloudflare; Cloudflare pulls from origin over HTTP/1.1, and forcing `http2` (h2c
  prior-knowledge) here would break those pulls. Client-facing multiplexing is already
  handled upstream.
- **The auth `access_by_lua` blocks** — untouched; the 60s `token_cache` shared dict already
  makes the common path a local lookup with no backend round-trip.
- **`proxy_cache`** for API — left off; the app's stale-while-revalidate cache + the planned
  service worker own response caching, and caching authenticated JSON at nginx is risky
  (per-user data) without careful keying.

### Verify

```sh
nginx -t                                   # config parses
curl -H 'Accept-Encoding: gzip' -I https://…/_app/immutable/chunks/<hash>.js   # content-encoding: gzip
curl -N https://…/api/v1/events            # SSE frames still arrive live (not buffered)
```
Lighthouse mobile: "Enable text compression" passes; total transfer weight drops sharply.

---

## Suggested order & sizing

| # | Item | Layer | Risk | Effort | Perceived gain |
|---|------|-------|------|--------|----------------|
| N | **nginx phase — gzip+precompress, open_file_cache, buffering split** ✅ applied | serving | very low | S | **huge** (cold load + every API hit) |
| 1.2 | Preconnect image.tmdb.org ✅ applied | frontend | very low | XS | medium |
| 1.3 | Right-size poster sizes ✅ applied | frontend | low | S | **high** (lists) |
| 2.1 | Service worker (shell + SWR) | frontend | medium | M | **high** (relaunch) |
| 2.2 | Skeleton placeholders | frontend | low* | M | high (feel) |
| 2.3 | Audit instant cached paint | frontend | low | S | medium |
| 3.1 | Self-host/trim font | frontend | low | S | medium |
| 3.2 | Image decode/CLS hints ✅ applied (LCP-priority deferred) | frontend | very low | S | small–medium |
| 3.3 | Parallelize discover warm ✅ applied | frontend | low | XS | small |
| 3.4 | Backend Cache-Control | backend | low | S | medium (w/ 2.1) |
| 3.5 | View transitions | frontend | low | S | small (feel) |

\* 2.2 is the only item that adds markup (a loading state); everything else is invisible to
the rendered UI.

**Recommended path:** 1.1 → 1.2 → 1.3 (one afternoon, mostly serving + a URL helper) →
2.1 → 2.2 → the Tier 3 polish as time allows.

**Verification each step:** `npm run check` stays green; Lighthouse mobile before/after
(watch LCP, "Enable text compression," "Properly size images," total byte weight); manual
smoke on a throttled connection (DevTools "Fast 3G") to feel the difference. No design
tokens, classes, or markup change except 2.2.
