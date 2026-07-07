<script lang="ts">
  import { onMount } from 'svelte';
  import { Button } from '$lib/components/ui/button';
  import { Search, Download, Calendar, ExternalLink, CheckCircle2, Share2 } from 'lucide-svelte';
  import { posterUrl } from '$lib/search/map';
  import { posterAtWidth, POSTER_THUMB_WIDTH } from '$lib/utils/poster-url';
  import { canShare, share } from '$lib/utils/share';
  import type { MediaItem, MediaType } from '$lib/types/media';

  interface Props {
    item: MediaItem;
    mediaType: MediaType;
    available?: boolean;
    onSearch?: () => void;
    onDownload?: () => void;
  }

  let { item, mediaType, available = false, onSearch, onDownload }: Props = $props();

  const imageSrc = $derived(posterAtWidth(posterUrl(item.images.poster), POSTER_THUMB_WIDTH));

  // Resolve share support after mount (navigator isn't there during prerender).
  let shareSupported = $state(false);
  onMount(() => {
    shareSupported = canShare();
  });

  function onShare() {
    void share({
      title: item.title,
      text: `${item.title}${item.year ? ` (${item.year})` : ''}`,
      url: `${location.origin}/search?q=${encodeURIComponent(item.title)}`,
    });
  }
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
          width="44"
          height="64"
          decoding="async"
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
      {#if shareSupported}
        <Button
          onclick={onShare}
          size="sm"
          variant="outline"
          class="h-7 w-full px-2 text-xs"
          aria-label="Share {item.title}"
        >
          <Share2 class="mr-1 h-3 w-3" />
          Share
        </Button>
      {/if}
    </div>
  </div>
</div>
