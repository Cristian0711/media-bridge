<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { Button } from '$lib/components/ui/button';
  import { clearListCache } from '$lib/data/list-cache';
  import { clearPosterPreloadState } from '$lib/utils/poster-preload';
  import { clearToken } from '$lib/auth/session';
  import { getCurrentUser } from '$lib/auth/api';
  import { canInstall, promptInstall } from '$lib/pwa/install';
  import type { CurrentUser } from '$lib/types/auth';
  import { ChevronRight, Download, ShieldCheck } from 'lucide-svelte';

  let currentUser = $state<CurrentUser | null>(null);

  async function loadCurrentUser() {
    try {
      currentUser = await getCurrentUser();
    } catch {
      currentUser = null;
    }
  }

  onMount(() => {
    void loadCurrentUser();
  });

  function logout() {
    clearListCache();
    clearPosterPreloadState();
    clearToken();
    goto('/login');
  }
</script>

<div class="space-y-8 pb-8">
  {#if currentUser?.role === 'admin'}
    <section class="space-y-3">
      <h2 class="text-base font-semibold text-white">Administration</h2>
      <p class="text-sm text-muted-foreground">
        Manage system health, invite keys, and indexer configuration.
      </p>
      <button
        type="button"
        class="flex w-full items-center gap-3 rounded-xl border border-white/10 bg-card/60 px-4 py-3 text-left transition-colors hover:border-white/20"
        onclick={() => goto('/settings/admin')}
      >
        <ShieldCheck class="h-5 w-5 shrink-0 text-white/50" />
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium text-white">Admin panel</p>
          <p class="text-xs text-white/45">Health checks, invitations, and indexers</p>
        </div>
        <ChevronRight class="h-4 w-4 shrink-0 text-white/30" />
      </button>
    </section>
  {/if}

  {#if $canInstall}
    <section class="space-y-3 {currentUser?.role === 'admin' ? 'border-t border-white/10 pt-6' : ''}">
      <p class="text-sm text-muted-foreground">App</p>
      <button
        type="button"
        class="flex w-full items-center gap-3 rounded-xl border border-white/10 bg-card/60 px-4 py-3 text-left transition-colors hover:border-white/20"
        onclick={() => void promptInstall()}
      >
        <Download class="h-5 w-5 shrink-0 text-white/50" />
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium text-white">Install app</p>
          <p class="text-xs text-white/45">Add Media Bridge to your home screen</p>
        </div>
        <ChevronRight class="h-4 w-4 shrink-0 text-white/30" />
      </button>
    </section>
  {/if}

  <section class="space-y-3 {currentUser?.role === 'admin' || $canInstall ? 'border-t border-white/10 pt-6' : ''}">
    <p class="text-sm text-muted-foreground">Account</p>
    {#if currentUser}
      <p class="text-sm text-white">
        Signed in as <span class="font-medium">{currentUser.username}</span>
        {#if currentUser.role === 'admin'}
          <span class="text-white/45"> · admin</span>
        {/if}
      </p>
    {/if}
    <Button variant="outline" onclick={logout}>Sign out</Button>
  </section>
</div>
