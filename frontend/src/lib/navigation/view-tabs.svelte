<script lang="ts">
  import { cn } from '$lib/utils';
  import LiquidTabTrack from './liquid-tab-track.svelte';
  import type { User } from 'lucide-svelte';

  type Tab = { id: string; label: string; icon: typeof User };

  interface Props {
    tabs: Tab[];
    activeId: string;
    onSelect: (id: string) => void;
    ariaLabel: string;
  }

  let { tabs, activeId, onSelect, ariaLabel }: Props = $props();

  const activeIndex = $derived(tabs.findIndex((t) => t.id === activeId));
  let highlightedIndex = $state(0);
  let liquidTrack = $state<LiquidTabTrack | undefined>();

  function isHighlighted(id: string) {
    return tabs[highlightedIndex]?.id === id;
  }
</script>

<LiquidTabTrack
  bind:this={liquidTrack}
  bind:highlightedIndex
  activeIndex={activeIndex >= 0 ? activeIndex : 0}
  onCommit={(i) => onSelect(tabs[i].id)}
  class={cn(
    'grid w-full max-w-sm grid-cols-2 gap-1 rounded-full p-1.5',
    'border border-white/10 bg-white/[0.06] backdrop-blur-2xl backdrop-saturate-150',
    'shadow-[0_18px_40px_-12px_rgba(0,0,0,0.6),inset_0_1px_0_rgba(255,255,255,0.08)]',
  )}
  role="tablist"
  aria-label={ariaLabel}
>
  {#each tabs as tab (tab.id)}
    {@const highlighted = isHighlighted(tab.id)}
    <button
      type="button"
      role="tab"
      data-liquid-tab
      aria-selected={activeId === tab.id}
      class={cn(
        'relative z-10 inline-flex h-10 items-center justify-center gap-1.5 rounded-full text-sm font-medium transition-all duration-220 outline-none',
        'focus-visible:ring-[3px] focus-visible:ring-ring/50',
        highlighted
          ? 'text-white [@media(hover:hover)_and_(pointer:fine)]:bg-white/14 [@media(hover:hover)_and_(pointer:fine)]:shadow-[inset_0_1px_0_rgba(255,255,255,0.18),0_0_0_1px_rgba(255,255,255,0.06)]'
          : 'text-white/55 hover:text-white/80',
      )}
      onclick={() => {
        if (liquidTrack?.consumeScrubClick()) return;
        if (window.matchMedia('(pointer: coarse)').matches) return;
        onSelect(tab.id);
      }}
    >
      <tab.icon class="h-4 w-4" strokeWidth={highlighted ? 2.25 : 1.75} />
      <span class="truncate">{tab.label}</span>
    </button>
  {/each}
</LiquidTabTrack>
