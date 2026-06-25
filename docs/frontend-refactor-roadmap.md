# Frontend Refactor Roadmap

Goal: a cleaner, more reusable frontend **without any visual/design changes**. Every step
below is behavior-preserving — the rendered UI stays pixel-identical. We only consolidate
duplicated logic, extract reusable parts, fix folder/file placement, and tighten naming.

Stack: SvelteKit 2 + Svelte 5 (runes) + TypeScript + Tailwind 4 + bits-ui.

Verification after every step: `npm run check` (svelte-check) must stay clean, and a quick
manual smoke of the affected page. No design tokens, classes, or markup change.

---

## Guiding principles

- **One source of truth** for every pure helper (URL/size/time/quality formatting).
- **Domain owns its data**; `lib/data` owns caching; `lib/navigation` owns chrome only.
- **Extract a component only when it's used ≥2×** and the extraction doesn't change markup.
- Keep diffs mechanical and reviewable — small steps, each independently shippable.

---

## Phase 1 — Consolidate pure helpers (lowest risk, highest signal)

These are copy-pasted pure functions. Collapse to one definition each; update imports.

1.1 **Poster URL normalization** — 4 copies today:
- canonical: `lib/utils/poster-url.ts` → `normalizePosterUrl()`
- dupes: `lib/media/map.ts:posterUrl`, `lib/requests/map.ts:posterUrl`, `lib/search/map.ts:posterUrl`
- Action: re-export `normalizePosterUrl` (keep a `posterUrl` alias where callers expect it,
  or update call sites). `search/map.ts` keeps its placeholder fallback by wrapping the
  shared fn, not re-implementing the protocol logic.

1.2 **Relative-time formatting** — identical in `lib/media/map.ts` and `lib/requests/map.ts`.
- Action: move to `lib/utils/format-time.ts` → `formatRelativeTime()`; both maps import it.

1.3 **Byte/size formatting** — `formatSizeGB` (media/map), inline `formatSize` in
`movie-torrent-dialog.svelte` & `show-torrent-dialog.svelte`, and `formatBytes` in
`requests/torrent-format.ts`.
- Action: create `lib/utils/format-size.ts` with `formatBytes()` (full units) and
  `formatSizeGB()`. Indexer dialogs and torrent-format import from it; delete inline copies.

1.4 **Query-string builder** — `queryString`/`qs` repeated in `media/api.ts`,
`requests/list-api.ts`, `indexer/api.ts`.
- Action: add `lib/api/query.ts` → `buildQuery(params, { snakeCaseKeys })`; callers use it.

Outcome: ~120 lines of duplication removed; one place to fix each formatter.

---

## Phase 2 — Unify the HTTP layer

`browse/api.ts` and `search/api.ts` hand-roll `authFetch` + `parseJson` + `apiErrorFrom`,
bypassing `lib/api/client.ts:callApi` (so the 401→login redirect never fires for them).

2.1 Extend `callApi` to optionally expose response headers (browse/search read
`X-Search-Page` / `X-Search-Total-Pages`). Add a `callApiRaw` variant returning
`{ data, headers, status }`, or a `parseHeaders` callback option.

2.2 Rewrite `browse/api.ts` and `search/api.ts` to use `callApi`/`callApiRaw`. Delete the
local `authFetch`/`parseJson`/`apiErrorFrom`.

2.3 Drop the `@deprecated` browse list endpoints (`fetchBrowseServiceLists`,
`fetchBrowseGlobalLists`) if nothing references them (verify first).

Outcome: single fetch/error/401 path for the whole app.

---

## Phase 3 — Generic stores & view-tabs (kill the library/requests twins)

`library-ui.ts` ≡ `requests-ui.ts`, and `library-view-tabs.svelte` ≡ `requests-view-tabs.svelte`
(only labels/setter differ).

3.1 `lib/data/view-store.ts` → `createViewStore<T>(initial)` returning `{ store, setView }`
(with the `scrollPageToTop()` on change). Re-home the two view stores here as
`library-view.ts` / `requests-view.ts` (semantically data state, not navigation chrome).

3.2 `lib/navigation/view-tabs.svelte` → generic `<ViewTabs>` taking
`{ items, activeId, onSelect, ariaLabel }`. Delete the two twins; pages pass their config.

3.3 Update imports in `library/+page.svelte`, `requests/+page.svelte`, and any tab consumers.

Outcome: ~140 lines removed; adding a future tabbed view is one config object.

---

## Phase 4 — Auth pages & form card

`login/+page.svelte` and `register/+page.svelte` share structure, error handling, and footer.
`components/auth/auth-form-card.svelte` already exists — check how much it covers.

4.1 Push shared submit/error/redirect handling into `auth-form-card` (or a small
`useAuthForm` helper in `lib/auth/`). Pages become field config + submit callback.

Outcome: login/register shrink to ~10 lines each; one place for auth-form behavior.

---

## Phase 5 — Reusable presentational pieces (markup-preserving extractions)

Only extract where markup is byte-identical across uses, so the UI cannot shift.

5.1 **TorrentRow** — `movie-torrent-dialog` and `show-torrent-dialog` render the same
torrent card (title + download btn + badges + size/seeders/leechers stats).
`show-torrent-dialog` already has a `{#snippet torrentRow()}`; promote it to
`components/indexer/torrent-row.svelte` and use in both.

5.2 **QualityFilter** — identical filter grid in both indexer dialogs →
`components/indexer/quality-filter.svelte`.

5.3 **StatItem** — the icon+value stat used in torrent rows → tiny shared snippet/component.

Note: `media-card` / `media-library-item` / `request-card` share an outer card shell but
have genuinely different inner content and props. Defer a "base card" extraction unless we
can do it without touching markup — lower priority, evaluate last.

---

## Phase 6 — Page-logic extraction (thin the route components)

Routes carry heavy inline logic. Extract controllers/composables; markup stays in the route.

6.1 **Pagination + SSE + view-switch** pattern is duplicated between `library/+page.svelte`
(363 lines) and `requests/+page.svelte` (200 lines). Extract a `createPaginatedList(...)`
composable in `lib/data/` (cache load, page state, search, SSE live-update wiring, race-guard).

6.2 **`+page.svelte` (home/discover, 223 lines)** — extract catalog→row-state mapping
(`mergeRowState`, `listsFromCatalog`) into `lib/browse/`.

6.3 **`settings/+page.svelte` (249 lines)** — split admin invite-keys section into its own
component; keep account/health sections separate.

Outcome: route files become layout + wiring; logic is unit-testable and shared.

---

## Phase 7 — Type & naming consistency (cleanup pass)

7.1 `IndexerMovie`/`IndexerShow` and `SearchMoviePayload`/`SearchShowPayload` are ~90%
overlapping — introduce a shared base type, extend for the show-only fields.

7.2 Standardize DTO naming: pick `Api*` prefix for raw server shapes consistently
(`ApiMedia` exists; `RequestRow`, `QbitTorrent` don't follow it) — low-risk rename pass.

7.3 Decide on barrel `index.ts` exports per component folder (optional; only if it reduces
import churn after the moves above).

---

## Suggested order & sizing

| Phase | Risk | Est. churn | Ship independently? |
|-------|------|-----------|---------------------|
| 1 Helpers       | very low | small  | yes |
| 2 HTTP layer    | low      | medium | yes |
| 3 View twins    | low      | medium | yes |
| 4 Auth pages    | low      | small  | yes |
| 5 Torrent pieces| medium   | medium | yes |
| 6 Page logic    | medium   | large  | per-route |
| 7 Types/naming  | low      | medium | yes |

Recommended path: 1 → 2 → 3 → 4 → 5 → 7 → 6 (do the big page-logic extraction last, on a
clean base). Each phase ends with `npm run check` green and a manual smoke of touched pages.
