<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { haptic } from '$lib/utils/haptics';
  import LiquidGlassNav from './liquid-glass/liquid-glass-nav.svelte';
  import type { NavItem } from './liquid-glass/types';
  import { TABS, tabFromPath } from './tabs';

  const active = $derived(tabFromPath(page.url.pathname));

  const items: NavItem[] = TABS.map((t) => ({ id: t.id, label: t.label, icon: t.icon }));

  function onSelect(id: string) {
    const tab = TABS.find((t) => t.id === id);
    if (!tab) return;
    haptic();
    goto(tab.href);
  }
</script>

<LiquidGlassNav {items} activeId={active} {onSelect} />
