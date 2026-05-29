<script lang="ts">
  import { cn } from '$lib/utils';
  import type { HTMLButtonAttributes } from 'svelte/elements';
  import { type VariantProps, tv } from 'tailwind-variants';

  const buttonVariants = tv({
    base: cn(
      'inline-flex shrink-0 items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium outline-none transition-all',
      'focus-visible:ring-[3px] focus-visible:ring-ring/50',
      'disabled:pointer-events-none disabled:opacity-50',
      '[&_svg]:pointer-events-none [&_svg]:shrink-0',
    ),
    variants: {
      variant: {
        default: 'bg-primary text-primary-foreground hover:bg-primary/90',
        outline:
          'border border-white/15 bg-white/5 text-foreground hover:bg-white/10',
        destructive:
          'bg-red-600/90 text-white hover:bg-red-600',
        ghost: 'hover:bg-white/10 hover:text-foreground',
      },
      size: {
        default: 'h-9 px-4 py-2',
        sm: 'h-7 rounded-md px-2 text-xs',
        icon: 'size-9',
        'icon-lg': 'size-12',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  });

  type Props = HTMLButtonAttributes & {
    variant?: VariantProps<typeof buttonVariants>['variant'];
    size?: VariantProps<typeof buttonVariants>['size'];
    class?: string;
    children?: import('svelte').Snippet;
  };

  let {
    class: className,
    variant = 'default',
    size = 'default',
    children,
    ...restProps
  }: Props = $props();
</script>

<button class={cn(buttonVariants({ variant, size }), className)} {...restProps}>
  {@render children?.()}
</button>
