<script lang="ts">
  import { Button } from '$lib/components/ui/button';
  import { Search, Download, Calendar, ExternalLink, CheckCircle2 } from 'lucide-svelte';
  import { posterUrl } from '$lib/search/map';
  import type { MediaItem, MediaType } from '$lib/types/media';

  interface Props {
    item: MediaItem;
    mediaType: MediaType;
    available?: boolean;
    onSearch?: () => void;
    onDownload?: () => void;
  }

  let { item, mediaType, available = false, onSearch, onDownload }: Props = $props();

  const imageSrc = $derived(posterUrl(item.images.poster));
</script>

<div
  class="mb-2 overflow-hidden rounded-lg border border-border/40 bg-card transition-colors duration-200 hover:bg-muted/20"
>
  <div class="flex items-center gap-3 p-3">
    <div class="shrink-0">
      <div class="relative h-16 w-11 overflow-hidden rounded bg-muted">
        <img
          src={imageSrc}
          alt="{item.title} poster"
          class="h-full w-full object-cover"
          loading="lazy"
        />
      </div>
    </div>

    <div class="min-w-0 flex-1 space-y-0.5">
      <h3 class="line-clamp-1 text-sm leading-tight font-semibold">
        {item.title}
      </h3>

      <div class="flex items-center gap-1.5 text-xs text-muted-foreground">
        <Calendar class="h-3 w-3" />
        <span>{item.year}</span>
        <span class="ml-1">
          · {mediaType === 'movies' ? 'Movie' : 'TV Show'}
        </span>
        {#if available}
          <CheckCircle2 class="ml-1 h-3.5 w-3.5 text-green-500" aria-label="Already on your server" />
        {/if}
      </div>

      <div class="flex items-center gap-2 text-xs">
        {#if item.ids.imdb}
          <a
            href="https://www.imdb.com/title/{item.ids.imdb}"
            target="_blank"
            rel="noopener noreferrer"
            class="flex items-center gap-0.5 text-blue-400 hover:text-blue-300"
          >
            IMDB <ExternalLink class="h-2.5 w-2.5" />
          </a>
        {/if}
      </div>
    </div>

    <div class="flex shrink-0 flex-col gap-1.5">
      <Button onclick={onSearch} size="sm" variant="outline" class="h-7 px-2 text-xs">
        <Search class="mr-1 h-3 w-3" />
        Search
      </Button>
      <Button onclick={onDownload} size="sm" variant="outline" class="h-7 px-2 text-xs">
        <Download class="mr-1 h-3 w-3" />
        Download
      </Button>
    </div>
  </div>
</div>
