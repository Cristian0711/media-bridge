<script lang="ts">
  import type { Snippet } from 'svelte';
  import { Button } from '$lib/components/ui/button';
  import AuthFormCard from './auth-form-card.svelte';

  interface Props {
    title: string;
    subtitle: string;
    error: string;
    loading: boolean;
    submitLabel: string;
    loadingLabel: string;
    onSubmit: (event: SubmitEvent) => void;
    children: Snippet;
    footer: Snippet;
  }

  let {
    title,
    subtitle,
    error,
    loading,
    submitLabel,
    loadingLabel,
    onSubmit,
    children,
    footer,
  }: Props = $props();
</script>

<div
  class="flex min-h-0 flex-1 items-center justify-center overflow-hidden px-6 pb-[max(1.25rem,env(safe-area-inset-bottom))]"
>
  <AuthFormCard {title} {subtitle} {footer}>
    <form class="space-y-4" onsubmit={onSubmit}>
      {@render children()}

      {#if error}
        <p class="text-sm text-red-400">{error}</p>
      {/if}

      <Button type="submit" class="w-full" disabled={loading}>
        {loading ? loadingLabel : submitLabel}
      </Button>
    </form>
  </AuthFormCard>
</div>
