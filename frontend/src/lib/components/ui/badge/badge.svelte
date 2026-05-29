<script lang="ts">
  import { cn } from '$lib/utils';
  import type { HTMLAttributes } from 'svelte/elements';

  type Props = HTMLAttributes<HTMLSpanElement> & {
    variant?: 'default' | 'secondary' | 'outline';
  };

  let { class: className, variant = 'default', children, ...rest }: Props = $props();
</script>

<span
  class={cn(
    'inline-flex items-center rounded-md border px-2 py-0.5 text-xs font-medium transition-colors',
    variant === 'default' && 'border-transparent bg-primary/20 text-primary',
    variant === 'secondary' && 'border-transparent bg-muted text-muted-foreground',
    variant === 'outline' && 'border-border text-foreground',
    className,
  )}
  {...rest}
>
  {@render children?.()}
</span>
