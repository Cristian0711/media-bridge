<script lang="ts">
  import { portal } from '$lib/utils/portal';
  import { lockBodyScroll } from '$lib/utils/scroll-lock';
  import { setDialogContext } from './context';
  import type { Snippet } from 'svelte';

  interface Props {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    children: Snippet;
  }

  let { open, onOpenChange, children }: Props = $props();

  // Let DialogContent dismiss itself (backdrop tap here, swipe-down there).
  setDialogContext({ close: () => onOpenChange(false) });

  $effect(() => {
    if (!open) return;
    return lockBodyScroll();
  });
</script>

{#if open}
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <!-- Docks to the bottom edge on touch (bottom sheet), centers on fine pointers. -->
  <div
    use:portal
    class="dialog-overlay fixed inset-0 z-[200] flex items-end justify-center overflow-hidden overscroll-none bg-black/60 [@media(pointer:fine)]:items-center [@media(pointer:fine)]:p-4"
    role="presentation"
    onclick={(e) => {
      if (e.target === e.currentTarget) onOpenChange(false);
    }}
  >
    {@render children()}
  </div>
{/if}
