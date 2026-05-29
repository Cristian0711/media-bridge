<script lang="ts">
  import { Film, Tv } from 'lucide-svelte';

  interface Props {
    src?: string;
    alt: string;
    /** First rows on screen — eager decode + no lazy deferral */
    priority?: boolean;
    fallback?: 'movie' | 'show';
  }

  let { src, alt, priority = false, fallback = 'movie' }: Props = $props();

  let visible = $state(false);

  function bindPoster(node: HTMLImageElement) {
    const reveal = () => {
      visible = true;
    };
    if (node.complete && node.naturalWidth > 0) {
      reveal();
    }
    node.addEventListener('load', reveal);
    return {
      destroy() {
        node.removeEventListener('load', reveal);
      },
    };
  }

  $effect(() => {
    src;
    visible = false;
  });
</script>

<div class="relative h-16 w-11 overflow-hidden rounded bg-muted">
  {#if src}
    <img
      {src}
      {alt}
      width="44"
      height="64"
      decoding="async"
      loading={priority ? 'eager' : 'lazy'}
      fetchpriority={priority ? 'high' : 'auto'}
      class="h-full w-full object-cover transition-opacity duration-200 {visible ? 'opacity-100' : 'opacity-0'}"
      use:bindPoster
    />
  {:else}
    <div class="flex h-full w-full items-center justify-center">
      {#if fallback === 'movie'}
        <Film class="h-5 w-5 text-muted-foreground" />
      {:else}
        <Tv class="h-5 w-5 text-muted-foreground" />
      {/if}
    </div>
  {/if}
</div>
