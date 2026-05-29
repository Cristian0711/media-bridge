<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    title: string;
    subtitle: string;
    children: Snippet;
    footer?: Snippet;
  }

  let { title, subtitle, children, footer }: Props = $props();
</script>

<div
  class="w-full max-w-sm rounded-xl border border-border/40 bg-card p-6 shadow-xl shadow-black/25"
>
  <h1 class="text-xl font-bold tracking-tight text-white">{title}</h1>
  <p class="mt-1 text-sm text-muted-foreground">{subtitle}</p>

  <div class="mt-6">
    {@render children()}
  </div>

  {#if footer}
    <div class="mt-6 border-t border-border/30 pt-5">
      {@render footer()}
    </div>
  {/if}
</div>

<style>
  :global(.auth-input) {
    width: 100%;
    border-radius: 0.75rem;
    border: 1px solid rgb(255 255 255 / 0.1);
    background: rgb(255 255 255 / 0.06);
    padding: 0.75rem 1rem;
    font-size: 16px;
    color: #fff;
    outline: none;
  }

  :global(.auth-input:focus) {
    border-color: oklch(0.78 0.13 75 / 0.5);
    box-shadow: 0 0 0 3px oklch(0.78 0.13 75 / 0.15);
  }

  :global(.auth-input::placeholder) {
    color: rgb(255 255 255 / 0.35);
  }
</style>
