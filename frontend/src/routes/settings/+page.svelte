<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { Button } from '$lib/components/ui/button';
  import { Badge } from '$lib/components/ui/badge';
  import { clearListCache } from '$lib/data/list-cache';
  import { clearPosterPreloadState } from '$lib/utils/poster-preload';
  import { clearToken } from '$lib/auth/session';
  import { generateRegistrationKey, getCurrentUser, listRegistrationKeys } from '$lib/auth/api';
  import { getLatestHealthScan } from '$lib/health/api';
  import type { CurrentUser, InviteKey } from '$lib/types/auth';
  import { overallStatusClass, overallStatusLabel } from '$lib/health/status';
  import type { ScanLogSummary } from '$lib/types/health-log';
  import { Activity, ChevronRight, ClipboardList, Copy, KeyRound, Loader2 } from 'lucide-svelte';

  let latest = $state<ScanLogSummary | null>(null);
  let loading = $state(true);
  let currentUser = $state<CurrentUser | null>(null);
  let generatedKey = $state<string | null>(null);
  let generatingKey = $state(false);
  let keyError = $state<string | null>(null);
  let copiedKey = $state(false);
  let showInviteKeys = $state(false);
  let inviteKeys = $state<InviteKey[]>([]);
  let loadingInviteKeys = $state(false);
  let inviteKeysError = $state<string | null>(null);

  async function loadLatest() {
    loading = true;
    try {
      const resp = await getLatestHealthScan();
      latest = resp.scan;
    } catch {
      latest = null;
    } finally {
      loading = false;
    }
  }

  async function loadCurrentUser() {
    try {
      currentUser = await getCurrentUser();
    } catch {
      currentUser = null;
    }
  }

  async function loadInviteKeys() {
    loadingInviteKeys = true;
    inviteKeysError = null;
    try {
      const resp = await listRegistrationKeys();
      inviteKeys = resp.keys;
    } catch (e) {
      inviteKeys = [];
      inviteKeysError = e instanceof Error ? e.message : 'Failed to load invite keys';
    } finally {
      loadingInviteKeys = false;
    }
  }

  async function toggleInviteKeys() {
    showInviteKeys = !showInviteKeys;
    if (showInviteKeys) {
      await loadInviteKeys();
    }
  }

  async function createRegistrationKey() {
    generatingKey = true;
    keyError = null;
    copiedKey = false;
    try {
      const resp = await generateRegistrationKey();
      generatedKey = resp.key;
      if (showInviteKeys) {
        await loadInviteKeys();
      }
    } catch (e) {
      generatedKey = null;
      keyError = e instanceof Error ? e.message : 'Failed to generate key';
    } finally {
      generatingKey = false;
    }
  }

  function inviteKeyStatusLabel(status: InviteKey['status']) {
    return status === 'available' ? 'Available' : 'Used';
  }

  function inviteKeyStatusClass(status: InviteKey['status']) {
    return status === 'available'
      ? 'border-emerald-500/30 text-emerald-300'
      : 'border-white/20 text-white/50';
  }

  async function copyRegistrationKey() {
    if (!generatedKey) return;
    try {
      await navigator.clipboard.writeText(generatedKey);
      copiedKey = true;
    } catch {
      copiedKey = false;
    }
  }

  onMount(() => {
    void loadLatest();
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
  <section class="space-y-3">
    <h2 class="text-base font-semibold text-white">System health</h2>
    <p class="text-sm text-muted-foreground">
      Automated scans every 10 minutes. Open the scan log for history and manual scans.
    </p>

    {#if loading}
      <div class="flex justify-center py-6 text-white/40">
        <Loader2 class="h-5 w-5 animate-spin" />
      </div>
    {:else if latest}
      <div
        class="flex items-center gap-3 rounded-xl border px-4 py-3 {overallStatusClass(latest.status)}"
      >
        <Activity class="h-5 w-5 shrink-0" />
        <div class="min-w-0 flex-1">
          <p class="font-medium">{overallStatusLabel(latest.status)}</p>
          <p class="text-xs opacity-80">
            Last scan {new Date(latest.checked_at).toLocaleString()}
            {#if latest.full_scan}
              · full
            {:else}
              · quick
            {/if}
            {#if latest.fail_count > 0}
              · {latest.fail_count} failed check(s)
            {/if}
          </p>
        </div>
        <Badge class="capitalize border-current/30 bg-transparent">{latest.status}</Badge>
      </div>
    {:else}
      <p class="text-sm text-muted-foreground">No scans yet. Use the scan log to run the first check.</p>
    {/if}

    <button
      type="button"
      class="flex w-full items-center gap-3 rounded-xl border border-white/10 bg-card/60 px-4 py-3 text-left transition-colors hover:border-white/20"
      onclick={() => goto('/settings/health-log')}
    >
      <ClipboardList class="h-5 w-5 shrink-0 text-white/50" />
      <div class="min-w-0 flex-1">
        <p class="text-sm font-medium text-white">View scan log</p>
        <p class="text-xs text-white/45">History, details, and run scan now</p>
      </div>
      <ChevronRight class="h-4 w-4 shrink-0 text-white/30" />
    </button>
  </section>

  {#if currentUser?.role === 'admin'}
    <section class="space-y-3 border-t border-white/10 pt-6">
      <h2 class="text-base font-semibold text-white">Invite keys</h2>
      <p class="text-sm text-muted-foreground">
        Generate a one-time registration key for a new user. Each key works once.
      </p>
      <div class="flex flex-col gap-2 sm:flex-row">
        <Button variant="outline" disabled={generatingKey} onclick={createRegistrationKey}>
          {#if generatingKey}
            <Loader2 class="mr-2 h-4 w-4 animate-spin" />
            Generating…
          {:else}
            <KeyRound class="mr-2 h-4 w-4" />
            Generate registration key
          {/if}
        </Button>
        <Button variant="outline" onclick={toggleInviteKeys}>
          <ClipboardList class="mr-2 h-4 w-4" />
          {showInviteKeys ? 'Hide invite keys' : 'View all invite keys'}
        </Button>
      </div>
      {#if showInviteKeys}
        {#if loadingInviteKeys}
          <div class="flex justify-center py-4 text-white/40">
            <Loader2 class="h-5 w-5 animate-spin" />
          </div>
        {:else if inviteKeysError}
          <p class="text-sm text-red-400">{inviteKeysError}</p>
        {:else if inviteKeys.length === 0}
          <p class="text-sm text-muted-foreground">No invite keys yet.</p>
        {:else}
          <ul class="space-y-2">
            {#each inviteKeys as item (item.value)}
              <li class="rounded-xl border border-white/10 bg-card/60 px-4 py-3">
                <div class="flex items-start justify-between gap-3">
                  <p class="min-w-0 flex-1 break-all font-mono text-sm text-white">{item.value}</p>
                  <Badge class="shrink-0 capitalize border bg-transparent {inviteKeyStatusClass(item.status)}">
                    {inviteKeyStatusLabel(item.status)}
                  </Badge>
                </div>
                <p class="mt-2 text-xs text-white/45">
                  Created {new Date(item.created_at).toLocaleString()}
                  {#if item.used_at}
                    · Used {new Date(item.used_at).toLocaleString()}
                  {/if}
                </p>
              </li>
            {/each}
          </ul>
        {/if}
      {/if}
      {#if keyError}
        <p class="text-sm text-red-400">{keyError}</p>
      {/if}
      {#if generatedKey}
        <div class="rounded-xl border border-white/10 bg-card/60 px-4 py-3">
          <p class="text-xs text-white/45">New registration key</p>
          <p class="mt-1 break-all font-mono text-sm text-white">{generatedKey}</p>
          <Button variant="ghost" size="sm" class="mt-2" onclick={copyRegistrationKey}>
            <Copy class="mr-2 h-4 w-4" />
            {copiedKey ? 'Copied' : 'Copy key'}
          </Button>
        </div>
      {/if}
    </section>
  {/if}

  <section class="space-y-3 border-t border-white/10 pt-6">
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
