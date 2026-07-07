<script lang="ts">
  import { cn } from '$lib/utils';
  import { haptic } from '$lib/utils/haptics';
  import { getDialogContext } from './context';
  import type { Snippet } from 'svelte';
  import type { HTMLAttributes } from 'svelte/elements';

  type Props = HTMLAttributes<HTMLDivElement> & {
    children: Snippet;
    class?: string;
  };

  let { class: className, children, ...rest }: Props = $props();

  const ctx = getDialogContext();

  // Swipe-down-to-dismiss, driven from the grab handle so it never competes with
  // scrolling inside the sheet body. Touch-only (the handle is hidden on fine
  // pointers, where the dialog is a centered modal).
  const DISMISS = 100; // px drag past which release closes the sheet

  let dragY = $state(0);
  let dragging = $state(false);
  let startY = 0;
  let tracking = false;
  let armed = false;

  function onHandleDown(e: PointerEvent) {
    if (e.pointerType === 'mouse') return;
    startY = e.clientY;
    tracking = true;
    dragging = true;
    armed = false;
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
  }

  function onHandleMove(e: PointerEvent) {
    if (!tracking) return;
    dragY = Math.max(0, e.clientY - startY);
    if (!armed && dragY >= DISMISS) {
      armed = true;
      haptic();
    } else if (armed && dragY < DISMISS) {
      armed = false;
    }
  }

  function onHandleUp() {
    if (!tracking) return;
    tracking = false;
    dragging = false;
    if (dragY >= DISMISS) ctx?.close();
    dragY = 0;
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div
  class={cn(
    'dialog-panel mx-auto flex min-h-0 min-w-0 w-full max-w-[min(90vw,36rem)] max-h-[70vh] flex-col overflow-hidden rounded-xl border border-white/10 bg-card shadow-xl',
    // Bottom-sheet dressing on touch: full width, only the top corners rounded,
    // taller, and clear of the home-indicator via the safe-area inset.
    '[@media(pointer:coarse)]:w-full [@media(pointer:coarse)]:max-w-none [@media(pointer:coarse)]:max-h-[85vh] [@media(pointer:coarse)]:rounded-b-none [@media(pointer:coarse)]:rounded-t-2xl [@media(pointer:coarse)]:pb-[env(safe-area-inset-bottom)]',
    // Smooth the snap-back after a partial swipe (not while actively dragging).
    dragging ? '' : 'transition-transform duration-200 ease-out',
    className,
  )}
  style={dragY > 0 ? `transform: translateY(${dragY}px);` : ''}
  role="dialog"
  aria-modal="true"
  onclick={(e) => e.stopPropagation()}
  {...rest}
>
  <!-- Grab handle: touch-only, the swipe-to-dismiss origin. -->
  <div
    class="hidden shrink-0 cursor-grab touch-none justify-center pb-1 pt-2.5 [@media(pointer:coarse)]:flex"
    onpointerdown={onHandleDown}
    onpointermove={onHandleMove}
    onpointerup={onHandleUp}
    onpointercancel={onHandleUp}
    aria-hidden="true"
  >
    <span class="h-1 w-9 rounded-full bg-white/25"></span>
  </div>

  {@render children()}
</div>
