<script lang="ts">
  import { cn } from '$lib/utils';
  import type { Snippet } from 'svelte';
  import type { HTMLAttributes } from 'svelte/elements';

  type Props = HTMLAttributes<HTMLDivElement> & {
    children: Snippet;
    class?: string;
  };

  let { class: className, children, ...rest }: Props = $props();
</script>

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div
  class={cn(
    'mx-auto flex min-h-0 min-w-0 w-full max-w-[min(90vw,36rem)] max-h-[70vh] flex-col overflow-hidden rounded-xl border border-white/10 bg-card shadow-xl',
    className,
  )}
  role="dialog"
  onclick={(e) => e.stopPropagation()}
  {...rest}
>
  {@render children()}
</div>
