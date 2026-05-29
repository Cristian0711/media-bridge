import { Clapperboard, ClipboardList, Home, Search, Settings } from 'lucide-svelte';

export type TabId = 'home' | 'search' | 'library' | 'requests' | 'settings';

/** Lucide icons are SvelteComponentTyped classes; use one icon as the shared type. */
type TabIcon = typeof Home;

export type Tab = {
  id: TabId;
  label: string;
  href: string;
  icon: TabIcon;
};

export const TABS: Tab[] = [
  { id: 'home', label: 'Home', href: '/', icon: Home },
  { id: 'search', label: 'Search', href: '/search', icon: Search },
  { id: 'library', label: 'Library', href: '/library', icon: Clapperboard },
  { id: 'requests', label: 'Requests', href: '/requests', icon: ClipboardList },
  { id: 'settings', label: 'Settings', href: '/settings', icon: Settings },
];

export function tabFromPath(pathname: string): TabId {
  if (pathname.startsWith('/search')) return 'search';
  if (pathname.startsWith('/library')) return 'library';
  if (pathname.startsWith('/requests')) return 'requests';
  if (pathname.startsWith('/settings')) return 'settings';
  return 'home';
}
