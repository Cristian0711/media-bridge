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

<div class={cn('space-y-1', className)} {...rest}>
  {@render children()}
</div>
