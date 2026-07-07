<script lang="ts">
  import { cn } from '$lib/utils';
  import type { HTMLAttributes } from 'svelte/elements';

  type Props = HTMLAttributes<HTMLDivElement> & { class?: string };

  let { class: className, ...rest }: Props = $props();
</script>

<!-- Subtle pulsing placeholder. bg is white-tinted (not bg-muted) so it reads on
     the dark card surface, which is itself near-black. -->
<div class={cn('animate-pulse rounded-md bg-white/[0.06]', className)} aria-hidden="true" {...rest}></div>
