<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { Button } from '$lib/components/ui/button';
  import { Badge } from '$lib/components/ui/badge';
  import AdminInviteKeys from '$lib/components/settings/admin-invite-keys.svelte';
  import IndexerSettings from '$lib/components/settings/indexer-settings.svelte';
  import { getCurrentUser } from '$lib/auth/api';
  import { getLatestHealthScan } from '$lib/health/api';
  import type { CurrentUser } from '$lib/types/auth';
  import { overallStatusClass, overallStatusLabel } from '$lib/health/status';
  import type { ScanLogSummary } from '$lib/types/health-log';
  import { Activity, ArrowLeft, ChevronRight, ClipboardList, Loader2 } from 'lucide-svelte';

  let currentUser = $state<CurrentUser | null>(null);
  let checkingAccess = $state(true);
  let latest = $state<ScanLogSummary | null>(null);
  let loadingHealth = $state(true);

  async function loadHealth() {
    loadingHealth = true;
    try {
      const resp = await getLatestHealthScan();
      latest = resp.scan;
    } catch {
      latest = null;
    } finally {
      loadingHealth = false;
    }
  }

  onMount(async () => {
    try {
      currentUser = await getCurrentUser();
    } catch {
      currentUser = null;
    }
    checkingAccess = false;
    if (currentUser?.role !== 'admin') {
      goto('/settings');
      return;
    }
    void loadHealth();
  });
</script>

<div class="space-y-8 pb-8">
  <div class="flex items-center gap-3">
    <Button variant="outline" size="sm" onclick={() => goto('/settings')}>
      <ArrowLeft class="mr-1.5 h-3.5 w-3.5" />
      Settings
    </Button>
    <h1 class="text-lg font-semibold text-white">Admin panel</h1>
  </div>

  {#if checkingAccess}
    <div class="flex justify-center py-10 text-white/40">
      <Loader2 class="h-6 w-6 animate-spin" />
    </div>
  {:else if currentUser?.role === 'admin'}
    <section class="space-y-3">
      <h2 class="text-base font-semibold text-white">System health</h2>
      <p class="text-sm text-muted-foreground">
        Automated scans every 10 minutes. Open the scan log for history and manual scans.
      </p>

      {#if loadingHealth}
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
        <p class="text-sm text-muted-foreground">
          No scans yet. Use the scan log to run the first check.
        </p>
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

    <AdminInviteKeys />

    <IndexerSettings />
  {/if}
</div>
