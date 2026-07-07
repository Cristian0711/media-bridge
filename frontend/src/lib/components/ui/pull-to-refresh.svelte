<script lang="ts">
  import { RefreshCw } from 'lucide-svelte';
  import { haptic } from '$lib/utils/haptics';
  import type { Snippet } from 'svelte';

  interface Props {
    /** Invoked when the user pulls past the threshold. Await it to keep the
        spinner up until the refresh completes. */
    onRefresh: () => void | Promise<void>;
    /** Skip the gesture entirely (e.g. while a text input / keyboard is up). */
    disabled?: boolean;
    children: Snippet;
  }

  let { onRefresh, disabled = false, children }: Props = $props();

  const THRESHOLD = 72; // px of pull needed to trigger a refresh
  const MAX = 96; // px cap on how far the indicator travels
  const RESIST = 0.5; // finger-to-pull ratio (rubber-band feel)

  let container = $state<HTMLElement | null>(null);
  let pull = $state(0);
  let refreshing = $state(false);
  let dragging = $state(false);

  let startY = 0;
  let tracking = false;
  let armed = false; // crossed the threshold; haptic already fired

  // Body is the page scroller (html is overflow:hidden) — check both to be safe.
  function atTop() {
    return (document.scrollingElement?.scrollTop ?? 0) <= 0 && document.body.scrollTop <= 0;
  }

  function coarsePointer() {
    return window.matchMedia('(pointer: coarse)').matches;
  }

  function onTouchStart(e: TouchEvent) {
    if (disabled || refreshing || e.touches.length !== 1 || !coarsePointer() || !atTop()) return;
    startY = e.touches[0].clientY;
    tracking = true;
    armed = false;
  }

  function onTouchMove(e: TouchEvent) {
    if (!tracking) return;
    const dy = e.touches[0].clientY - startY;

    // Pulling up, or the list scrolled off the top mid-gesture → hand back to scroll.
    if (dy <= 0 || !atTop()) {
      tracking = false;
      dragging = false;
      pull = 0;
      return;
    }

    dragging = true;
    pull = Math.min(MAX, dy * RESIST);
    // We own the gesture now — suppress native overscroll / browser pull-to-refresh.
    e.preventDefault();

    if (!armed && pull >= THRESHOLD) {
      armed = true;
      haptic();
    } else if (armed && pull < THRESHOLD) {
      armed = false;
    }
  }

  async function onTouchEnd() {
    if (!tracking) return;
    tracking = false;
    dragging = false;

    if (pull >= THRESHOLD) {
      refreshing = true;
      pull = THRESHOLD;
      try {
        await onRefresh();
      } finally {
        refreshing = false;
        pull = 0;
      }
    } else {
      pull = 0;
    }
  }

  // touchmove must be a non-passive listener for preventDefault() to take effect.
  $effect(() => {
    const el = container;
    if (!el) return;
    const opts = { passive: false } as AddEventListenerOptions;
    el.addEventListener('touchstart', onTouchStart, { passive: true });
    el.addEventListener('touchmove', onTouchMove, opts);
    el.addEventListener('touchend', onTouchEnd, { passive: true });
    el.addEventListener('touchcancel', onTouchEnd, { passive: true });
    return () => {
      el.removeEventListener('touchstart', onTouchStart);
      el.removeEventListener('touchmove', onTouchMove);
      el.removeEventListener('touchend', onTouchEnd);
      el.removeEventListener('touchcancel', onTouchEnd);
    };
  });

  const progress = $derived(Math.min(1, pull / THRESHOLD));
</script>

<div bind:this={container} class="relative">
  <div
    class="pointer-events-none absolute inset-x-0 top-0 z-10 flex justify-center"
    style="transform: translateY({pull - 36}px); opacity: {progress};"
    aria-hidden="true"
  >
    <div class="rounded-full border border-border/40 bg-card p-2 shadow-lg">
      <RefreshCw
        class="h-4 w-4 text-white {refreshing ? 'animate-spin' : ''}"
        style={refreshing ? '' : `transform: rotate(${pull * 3}deg)`}
      />
    </div>
  </div>

  <div
    style="transform: translateY({refreshing ? THRESHOLD * 0.4 : pull * 0.4}px);"
    class={dragging ? '' : 'transition-transform duration-200 ease-out'}
  >
    {@render children()}
  </div>
</div>
