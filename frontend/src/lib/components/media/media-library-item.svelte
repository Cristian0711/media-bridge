<script lang="ts">
  import { Button } from '$lib/components/ui/button';
  import { ExternalLink, Trash2 } from 'lucide-svelte';
  import PosterThumb from '$lib/components/media/poster-thumb.svelte';
  import { formatRelativeTime, formatSizeGB, mediaDetail, mediaTypeLabel } from '$lib/media/map';
  import { posterAtWidth, POSTER_THUMB_WIDTH } from '$lib/utils/poster-url';
  import type { MediaLibraryItem } from '$lib/types/media-library';

  interface Props {
    item: MediaLibraryItem;
    onRemove?: () => void;
    /** Row index in list — first rows load posters eagerly */
    index?: number;
  }

  let { item, onRemove, index = 99 }: Props = $props();

  const imageSrc = $derived(posterAtWidth(item.poster_url, POSTER_THUMB_WIDTH));
  const sizeLabel = $derived(formatSizeGB(item.size_bytes));

</script>

<div
  class="mb-2 overflow-hidden rounded-lg border border-border/40 bg-card transition-colors duration-200 hover:bg-muted/20"
>
  <div class="flex items-start gap-3 p-3">
    <div class="shrink-0">
      <PosterThumb
        src={imageSrc}
        alt="{item.title} poster"
        priority={index < 8}
        fallback={item.type === 'movie' ? 'movie' : 'show'}
      />
    </div>

    <div class="min-w-0 flex-1 space-y-1.5">
      <div class="flex items-center gap-2">
        <h4 class="truncate text-sm font-semibold leading-tight">{item.title}</h4>
        {#if mediaDetail(item)}
          <span class="rounded bg-white/10 px-1.5 py-0.5 text-[10px] font-medium text-white/75">
            {mediaDetail(item)}
          </span>
        {/if}
      </div>

      <div class="flex flex-wrap items-center gap-2 text-[11px] text-white/80">
        <span>{mediaTypeLabel(item.type)}</span>
        <span class="text-white/50">·</span>
        <span>{item.quality}</span>
        {#if sizeLabel}
          <span class="text-white/50">·</span>
          <span>{sizeLabel}</span>
        {/if}
        <span class="text-white/50">·</span>
        <span>@{item.username}</span>
        {#if item.imdb_id}
          <span>·</span>
          <a
            href="https://www.imdb.com/title/{item.imdb_id}"
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex items-center gap-0.5 text-blue-400 hover:text-blue-300"
          >
            IMDb <ExternalLink class="h-2.5 w-2.5" />
          </a>
        {/if}
      </div>
    </div>

    <div class="flex shrink-0 flex-col items-end gap-2">
      {#if onRemove}
        <Button
          variant="outline"
          size="sm"
          onclick={onRemove}
          class="h-7 w-7 p-0 text-red-400 hover:bg-red-500/10 hover:text-red-300"
          aria-label="Request removal"
        >
          <Trash2 class="h-3.5 w-3.5" />
        </Button>
      {/if}
      <span class="text-[10px] text-white/65">{formatRelativeTime(item.created_at)}</span>
    </div>
  </div>
</div>
