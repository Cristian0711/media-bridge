<script lang="ts">
  import { Button } from '$lib/components/ui/button';
  import { Download, Search, CheckCircle2 } from 'lucide-svelte';
  import { posterUrl, toMediaItem } from '$lib/search/map';
  import { posterAtWidth, POSTER_CARD_WIDTH } from '$lib/utils/poster-url';
  import type { MediaRow } from '$lib/media/media-action-host.svelte';
  import type { SearchResult } from '$lib/types/search';

  interface Props {
    title: string;
    loading?: boolean;
    error?: string;
    results?: SearchResult[];
    onSearch: (row: MediaRow) => void;
    onDownload: (row: MediaRow) => void;
  }

  let { title, loading = false, error = '', results = [], onSearch, onDownload }: Props = $props();

  const rows = $derived(
    results.map(toMediaItem).filter((r): r is NonNullable<typeof r> => r !== null),
  );
</script>

<section class="mb-6">
  <h2 class="mb-2 px-1 text-sm font-semibold tracking-tight">{title}</h2>

  {#if loading}
    <p class="px-1 text-xs text-muted-foreground">Loading…</p>
  {:else if error}
    <p class="px-1 text-xs text-red-400">{error}</p>
  {:else if rows.length === 0}
    <p class="px-1 text-xs text-muted-foreground">Nothing to show right now.</p>
  {:else}
    <div class="-mx-1 flex gap-3 overflow-x-auto px-1 pb-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
      {#each rows as row, i (`${row.mediaType}-${row.item.ids.tmdb ?? i}`)}
        {@const available = row.available}
        <article class="w-[7.5rem] shrink-0">
          <div class="flex h-[19rem] flex-col overflow-hidden rounded-lg border border-border/40 bg-card">
            <div class="relative h-[11.25rem] w-full shrink-0 overflow-hidden bg-muted">
              <img
                src={posterAtWidth(posterUrl(row.item.images.poster), POSTER_CARD_WIDTH)}
                alt="{row.item.title} poster"
                width="120"
                height="180"
                decoding="async"
                class="h-full w-full object-cover"
                loading="lazy"
              />
              {#if available}
                <div class="absolute right-1 top-1 rounded-full bg-black/60 p-0.5 backdrop-blur-sm">
                  <CheckCircle2 class="h-4 w-4 text-green-400" aria-label="Already on your server" />
                </div>
              {/if}
            </div>
            <div class="flex h-[7.75rem] flex-col gap-1 p-2">
              <p
                class="line-clamp-2 h-9 shrink-0 text-[11px] leading-[1.125rem] font-medium"
                title={row.item.title}
              >
                {row.item.title}
              </p>
              <p class="shrink-0 text-[10px] leading-none text-muted-foreground">{row.item.year}</p>
              <div class="mt-auto flex flex-col gap-1">
                <Button
                  onclick={() => onSearch(row)}
                  size="sm"
                  variant="outline"
                  class="h-6 w-full px-1 text-[10px]"
                >
                  <Search class="mr-0.5 h-2.5 w-2.5" />
                  Search
                </Button>
                <Button
                  onclick={() => onDownload(row)}
                  size="sm"
                  variant="outline"
                  class="h-6 w-full px-1 text-[10px]"
                >
                  <Download class="mr-0.5 h-2.5 w-2.5" />
                  Download
                </Button>
              </div>
            </div>
          </div>
        </article>
      {/each}
    </div>
  {/if}
</section>
