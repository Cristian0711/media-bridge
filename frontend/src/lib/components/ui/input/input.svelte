<script lang="ts">
  import { cn } from '$lib/utils';
  import type { HTMLInputAttributes } from 'svelte/elements';

  type Props = HTMLInputAttributes & { class?: string };

  let { class: className, value = $bindable(), ...rest }: Props = $props();
</script>

<input
  class={cn(
    'flex h-9 w-full rounded-md border border-white/10 bg-white/5 px-3 py-1 text-base text-foreground outline-none',
    'placeholder:text-muted-foreground focus-visible:ring-[3px] focus-visible:ring-ring/50',
    className,
  )}
  bind:value
  {...rest}
/>