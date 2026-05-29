<script lang="ts">
  import { cn } from '$lib/utils';
  import type { Snippet } from 'svelte';
  import type { HTMLAttributes } from 'svelte/elements';

  type Props = HTMLAttributes<HTMLHeadingElement> & {
    children: Snippet;
    class?: string;
  };

  let { class: className, children, ...rest }: Props = $props();
</script>

<h2 class={cn('text-base font-bold leading-tight', className)} {...rest}>
  {@render children()}
</h2>
