<script lang="ts">
  import type { Snippet } from 'svelte';
  import type { HTMLAttributes } from 'svelte/elements';
  import { cn } from '$lib/utils';
  import { bubbleMetricsAtX, tabRects } from './liquid-tab-geometry';

  type Props = {
    activeIndex: number;
    onCommit: (index: number) => void;
    class?: string;
    /** Index used for highlight while scrubbing; mirrors active when idle. */
    highlightedIndex?: number;
    children: Snippet;
  } & HTMLAttributes<HTMLDivElement>;

  let {
    activeIndex,
    onCommit,
    class: className,
    highlightedIndex = $bindable(),
    children,
    ...rest
  }: Props = $props();

  let trackEl = $state<HTMLDivElement | null>(null);
  let scrubbing = $state(false);
  let previewIndex = $state<number | null>(null);
  let bubbleLeft = $state(0);
  let bubbleWidth = $state(0);
  let bubbleReady = $state(false);
  let touchScrubEnabled = $state(false);
  let didScrub = $state(false);
  let pointerId = $state<number | null>(null);

  const displayIndex = $derived(
    scrubbing && previewIndex !== null ? previewIndex : activeIndex,
  );

  $effect(() => {
    highlightedIndex = displayIndex;
  });

  $effect(() => {
    const mq = window.matchMedia('(pointer: coarse)');
    touchScrubEnabled = mq.matches;
    const onChange = () => {
      touchScrubEnabled = mq.matches;
    };
    mq.addEventListener('change', onChange);
    return () => mq.removeEventListener('change', onChange);
  });

  $effect(() => {
    if (!trackEl || !touchScrubEnabled || scrubbing) return;
    activeIndex;
    queueMicrotask(() => placeBubbleAtIndex(activeIndex, false));
  });

  function placeBubbleAtIndex(index: number, fromScrub: boolean) {
    if (!trackEl) return;
    const rects = tabRects(trackEl);
    const r = rects[index];
    if (!r) return;
    bubbleLeft = r.left;
    bubbleWidth = r.width;
    bubbleReady = true;
    if (!fromScrub) previewIndex = null;
  }

  function placeBubbleAtClientX(clientX: number) {
    if (!trackEl) return;
    const trackBox = trackEl.getBoundingClientRect();
    const rects = tabRects(trackEl);
    const { left, width, previewIndex: idx } = bubbleMetricsAtX(rects, clientX, trackBox.left);
    bubbleLeft = left;
    bubbleWidth = width;
    previewIndex = idx;
    bubbleReady = true;
  }

  function startScrub(e: PointerEvent) {
    if (!touchScrubEnabled || e.pointerType !== 'touch' || !trackEl) return;
    pointerId = e.pointerId;
    scrubbing = true;
    didScrub = false;
    previewIndex = activeIndex;
    trackEl.setPointerCapture(e.pointerId);
    placeBubbleAtClientX(e.clientX);
    e.preventDefault();
  }

  function moveScrub(e: PointerEvent) {
    if (!scrubbing || e.pointerId !== pointerId) return;
    didScrub = true;
    placeBubbleAtClientX(e.clientX);
    e.preventDefault();
  }

  function endScrub(e: PointerEvent) {
    if (!scrubbing || e.pointerId !== pointerId) return;
    const commitIndex = previewIndex ?? activeIndex;
    scrubbing = false;
    pointerId = null;
    previewIndex = null;
    trackEl?.releasePointerCapture(e.pointerId);

    if (commitIndex !== activeIndex) {
      onCommit(commitIndex);
    }
    if (didScrub) e.preventDefault();

    queueMicrotask(() => placeBubbleAtIndex(commitIndex, false));
  }

  /** Suppress link navigation when the user scrubbed across tabs. */
  export function consumeScrubClick(): boolean {
    if (!didScrub) return false;
    didScrub = false;
    return true;
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
  {...rest}
  bind:this={trackEl}
  class={cn(
    'relative',
    touchScrubEnabled && scrubbing && 'touch-none select-none',
    className,
  )}
  onpointerdown={startScrub}
  onpointermove={moveScrub}
  onpointerup={endScrub}
  onpointercancel={endScrub}
  data-scrubbing={scrubbing ? '' : undefined}
>
  {#if touchScrubEnabled && bubbleReady}
    <div
      class={cn(
        'liquid-tab-bubble pointer-events-none absolute top-1.5 z-0',
        scrubbing ? 'liquid-tab-bubble--scrubbing' : 'liquid-tab-bubble--idle',
      )}
      style:left="{bubbleLeft}px"
      style:width="{bubbleWidth}px"
      aria-hidden="true"
    ></div>
  {/if}
  {@render children()}
</div>
