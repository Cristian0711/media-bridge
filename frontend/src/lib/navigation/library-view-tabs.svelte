<script lang="ts">
  import { cn } from '$lib/utils';
  import LiquidTabTrack from './liquid-tab-track.svelte';
  import { libraryView, setLibraryView, type LibraryView } from './library-ui';
  import { User, Users } from 'lucide-svelte';

  const tabs: { id: LibraryView; label: string; icon: typeof User }[] = [
    { id: 'yours', label: 'Your Media', icon: User },
    { id: 'all', label: 'All Media', icon: Users },
  ];

  const activeIndex = $derived(tabs.findIndex((t) => t.id === $libraryView));
  let highlightedIndex = $state(0);
  let liquidTrack = $state<LiquidTabTrack | undefined>();

  function isHighlighted(id: LibraryView) {
    return tabs[highlightedIndex]?.id === id;
  }
</script>

<LiquidTabTrack
  bind:this={liquidTrack}
  bind:highlightedIndex
  activeIndex={activeIndex >= 0 ? activeIndex : 0}
  onCommit={(i) => setLibraryView(tabs[i].id)}
  class={cn(
    'grid w-full max-w-sm grid-cols-2 gap-1 rounded-full p-1.5',
    'border border-white/10 bg-white/[0.06] backdrop-blur-2xl backdrop-saturate-150',
    'shadow-[0_18px_40px_-12px_rgba(0,0,0,0.6),inset_0_1px_0_rgba(255,255,255,0.08)]',
  )}
  role="tablist"
  aria-label="Library view"
>
  {#each tabs as tab (tab.id)}
    {@const highlighted = isHighlighted(tab.id)}
    <button
      type="button"
      role="tab"
      data-liquid-tab
      aria-selected={$libraryView === tab.id}
      class={cn(
        'relative z-10 inline-flex h-10 items-center justify-center gap-1.5 rounded-full text-sm font-medium transition-all duration-220 outline-none',
        'focus-visible:ring-[3px] focus-visible:ring-ring/50',
        highlighted
          ? 'text-white [@media(hover:hover)_and_(pointer:fine)]:bg-white/14 [@media(hover:hover)_and_(pointer:fine)]:shadow-[inset_0_1px_0_rgba(255,255,255,0.18),0_0_0_1px_rgba(255,255,255,0.06)]'
          : 'text-white/55 hover:text-white/80',
      )}
      onclick={(e) => {
        if (liquidTrack?.consumeScrubClick()) return;
        if (window.matchMedia('(pointer: coarse)').matches) return;
        setLibraryView(tab.id);
      }}
    >
      <tab.icon class="h-4 w-4" strokeWidth={highlighted ? 2.25 : 1.75} />
      {tab.label}
    </button>
  {/each}
</LiquidTabTrack>
