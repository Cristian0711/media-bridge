<script lang="ts">
  import '../app.css';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { browser } from '$app/environment';
  import AppHeader from '$lib/components/app-header.svelte';
  import BottomChrome from '$lib/navigation/bottom-chrome.svelte';
  import { TABS } from '$lib/navigation/tabs';
  import { clearSearchInputFocused } from '$lib/navigation/search-ui.svelte';
  import { validateToken } from '$lib/auth/api';
  import { clearToken, isAuthenticated, syncTokenFromCookie } from '$lib/auth/session';
  import { connectAppEvents } from '$lib/sse/client';
  import { sseConnectionStatus } from '$lib/sse/connection-status';
  import { syncListsAfterSseReconnect } from '$lib/sse/reconnect-sync';
  import ToastHost from '$lib/toast/toast-host.svelte';
  import { get } from 'svelte/store';

  let { children } = $props();

  const publicPaths = ['/login', '/register'];

  const isAuthPage = $derived(publicPaths.includes(page.url.pathname));

  onMount(() => {
    syncTokenFromCookie();
    const standalone =
      window.matchMedia('(display-mode: standalone)').matches ||
      (navigator as Navigator & { standalone?: boolean }).standalone === true;
    if (standalone) {
      document.documentElement.classList.add('ios-standalone');
    }
  });

  $effect(() => {
    if (!browser) return;

    const path = page.url.pathname;

    if (!isAuthenticated() && !publicPaths.includes(path)) {
      const redirect = encodeURIComponent(path + page.url.search);
      goto(`/login?redirect=${redirect}`);
      return;
    }

    if (isAuthenticated() && publicPaths.includes(path)) {
      void validateToken()
        .then((res) => {
          if (res.valid) goto('/');
        })
        .catch(() => clearToken());
    }
  });

  const onSearch = $derived(page.url.pathname.startsWith('/search'));
  const onLibrary = $derived(page.url.pathname.startsWith('/library'));
  const onRequests = $derived(page.url.pathname.startsWith('/requests'));
  const onHome = $derived(page.url.pathname === '/');
  const hidePageTitle = $derived(onHome || onSearch || onLibrary || onRequests);
  const chromeStacked = $derived(onLibrary || onRequests);

  const activeLabel = $derived(
    TABS.find((t) => page.url.pathname === t.href || (t.href !== '/' && page.url.pathname.startsWith(t.href)))
      ?.label ?? 'Home',
  );

  $effect(() => {
    if (!onSearch && !onLibrary) {
      clearSearchInputFocused();
    }
  });

  // One SSE connection per login session — do not depend on route path or we reconnect on every tab.
  $effect(() => {
    if (!browser || !isAuthenticated()) return;

    const conn = connectAppEvents();
    return () => conn.close();
  });

  // iOS PWA: returning from background may leave lists stale before SSE reconnect completes.
  $effect(() => {
    if (!browser || !isAuthenticated()) return;

    const MIN_AWAY_MS = 3000;
    let hiddenAt = 0;

    const onVisibility = () => {
      if (document.visibilityState === 'hidden') {
        hiddenAt = Date.now();
        return;
      }
      if (!hiddenAt) return;
      const away = Date.now() - hiddenAt;
      hiddenAt = 0;
      if (away < MIN_AWAY_MS) return;

      if (get(sseConnectionStatus) === 'connected') {
        syncListsAfterSseReconnect();
      }
    };

    document.addEventListener('visibilitychange', onVisibility);
    return () => document.removeEventListener('visibilitychange', onVisibility);
  });
</script>

<AppHeader />

{#if isAuthPage}
  <div class="auth-shell bg-surface pt-app-header">
    {@render children()}
  </div>
{:else}
  <div class="relative flex min-h-dvh flex-col bg-surface pt-app-header">
    <main
      class="flex flex-1 flex-col px-6 pt-4 {chromeStacked ? 'pb-[15.5rem]' : 'pb-52'}"
    >
      {#if !hidePageTitle}
        <h1 class="text-3xl font-bold tracking-tight text-white">{activeLabel}</h1>
      {/if}
      <div class={hidePageTitle ? 'flex-1' : 'mt-6 flex-1'}>
        {@render children()}
      </div>
    </main>

    <ToastHost />
    <BottomChrome />
  </div>
{/if}
