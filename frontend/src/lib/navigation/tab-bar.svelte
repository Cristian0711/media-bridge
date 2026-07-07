<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { cn } from '$lib/utils';
  import { haptic } from '$lib/utils/haptics';
  import LiquidTabTrack from './liquid-tab-track.svelte';
  import { TABS, tabFromPath, type TabId } from './tabs';

  const active = $derived(tabFromPath(page.url.pathname));
  const activeIndex = $derived(TABS.findIndex((t) => t.id === active));

  let highlightedIndex = $state(0);
  let liquidTrack = $state<LiquidTabTrack | undefined>();

  function isHighlighted(id: TabId) {
    return TABS[highlightedIndex]?.id === id;
  }

  function commitTab(index: number) {
    const tab = TABS[index];
    if (!tab) return;
    haptic();
    goto(tab.href);
  }
</script>

<LiquidTabTrack
  bind:this={liquidTrack}
  bind:highlightedIndex
  activeIndex={activeIndex >= 0 ? activeIndex : 0}
  onCommit={commitTab}
  class={cn(
    'flex max-w-full items-center gap-0.5 rounded-full p-1.5',
    'border border-white/10 bg-white/[0.06] backdrop-blur-2xl backdrop-saturate-150',
    'shadow-[0_18px_40px_-12px_rgba(0,0,0,0.6),inset_0_1px_0_rgba(255,255,255,0.08)]',
  )}
>
  {#each TABS as tab (tab.id)}
    {@const highlighted = isHighlighted(tab.id)}
    <a
      href={tab.href}
      data-liquid-tab
      data-sveltekit-preload-code="tap"
      aria-label={tab.label}
      aria-current={active === tab.id ? 'page' : undefined}
      class={cn(
        'relative z-10 inline-flex h-12 w-14 items-center justify-center rounded-full outline-none transition-all duration-220 sm:h-14 sm:w-[3.25rem]',
        'focus-visible:ring-[3px] focus-visible:ring-ring/50',
        highlighted
          ? 'text-white [@media(hover:hover)_and_(pointer:fine)]:bg-white/14 [@media(hover:hover)_and_(pointer:fine)]:shadow-[inset_0_1px_0_rgba(255,255,255,0.18),0_0_0_1px_rgba(255,255,255,0.06)]'
          : 'text-white/55 hover:text-white/80',
      )}
      onclick={(e) => {
        if (liquidTrack?.consumeScrubClick()) {
          e.preventDefault();
          return;
        }
        if (window.matchMedia('(pointer: coarse)').matches) e.preventDefault();
      }}
    >
      <tab.icon class="size-6 sm:size-7" strokeWidth={highlighted ? 2.25 : 1.75} />
    </a>
  {/each}
</LiquidTabTrack>
