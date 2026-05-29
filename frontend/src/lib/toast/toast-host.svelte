<script lang="ts">
  import { toasts } from '$lib/toast/toast.svelte';
</script>

<div
  class="pointer-events-none fixed bottom-[9.5rem] left-1/2 z-[100] flex w-[min(100%,20rem)] -translate-x-1/2 flex-col items-center gap-2 px-4"
  aria-live="polite"
>
  {#each $toasts as toast (toast.id)}
    <p
      class="w-full rounded-xl border border-border/60 bg-card/95 px-4 py-2.5 text-center text-sm text-foreground shadow-lg backdrop-blur-sm"
    >
      {toast.message}
    </p>
  {/each}
</div>
