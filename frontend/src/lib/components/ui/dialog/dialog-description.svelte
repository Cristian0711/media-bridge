<script lang="ts">
  import { cn } from '$lib/utils';
  import type { Snippet } from 'svelte';
  import type { HTMLAttributes } from 'svelte/elements';

  type Props = HTMLAttributes<HTMLParagraphElement> & {
    children: Snippet;
    class?: string;
  };

  let { class: className, children, ...rest }: Props = $props();
</script>

<p class={cn('text-xs text-muted-foreground', className)} {...rest}>
  {@render children()}
</p>
