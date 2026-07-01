<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { Button } from '$lib/components/ui/button';
  import { Badge } from '$lib/components/ui/badge';
  import HealthReportView from '$lib/components/health/health-report-view.svelte';
  import { getHealthReport, listHealthScans } from '$lib/health/api';
  import { overallStatusClass } from '$lib/health/status';
  import type { HealthReport } from '$lib/types/health';
  import type { ScanLogSummary } from '$lib/types/health-log';
  import { ApiError } from '$lib/api/client';
  import {
    ArrowLeft,
    ChevronRight,
    ClipboardList,
    HardDrive,
    Loader2,
    RefreshCw,
  } from 'lucide-svelte';

  let scans = $state<ScanLogSummary[]>([]);
  let loading = $state(false);
  let scanning = $state(false);
  let fullScan = $state(false);
  let error = $state('');
  let scanError = $state('');
  let report = $state<HealthReport | null>(null);
  let page = $state(1);
  let totalPages = $state(1);

  async function load(p = 1) {
    loading = true;
    error = '';
    try {
      const resp = await listHealthScans(p, 40);
      scans = resp.scans ?? [];
      page = resp.page;
      totalPages = Math.max(1, resp.total_pages);
    } catch (e) {
      scans = [];
      error = e instanceof ApiError ? e.message : 'Failed to load scan log';
    } finally {
      loading = false;
    }
  }

  async function runScan(full: boolean) {
    scanning = true;
    fullScan = full;
    scanError = '';
    report = null;
    try {
      report = await getHealthReport(full, true);
      await load(1);
    } catch (e) {
      scanError = e instanceof ApiError ? e.message : 'Health check failed';
    } finally {
      scanning = false;
    }
  }

  onMount(() => {
    void load(1);
  });

  function formatWhen(iso: string): string {
    return new Date(iso).toLocaleString();
  }
</script>

<div class="space-y-5 pb-8">
  <div class="flex items-center gap-2">
    <Button variant="outline" size="sm" onclick={() => goto('/settings')}>
      <ArrowLeft class="mr-1.5 h-3.5 w-3.5" />
      Settings
    </Button>
  </div>

  <div>
    <h2 class="text-base font-semibold text-white">Health scan log</h2>
    <p class="text-sm text-muted-foreground">
      Automatic scans every 10 minutes (quick). Full filesystem audit hourly. Logs kept 14 days.
    </p>
  </div>

  <section class="space-y-3 rounded-xl border border-white/10 bg-card/40 p-4">
    <div class="flex items-center justify-between gap-3">
      <p class="text-sm font-medium text-white">Run scan now</p>
      <div class="flex shrink-0 gap-2">
        <Button variant="outline" size="sm" disabled={scanning} onclick={() => runScan(false)}>
          {#if scanning && !fullScan}
            <Loader2 class="mr-1.5 h-3.5 w-3.5 animate-spin" />
          {:else}
            <RefreshCw class="mr-1.5 h-3.5 w-3.5" />
          {/if}
          Quick
        </Button>
        <Button variant="outline" size="sm" disabled={scanning} onclick={() => runScan(true)}>
          {#if scanning && fullScan}
            <Loader2 class="mr-1.5 h-3.5 w-3.5 animate-spin" />
          {:else}
            <HardDrive class="mr-1.5 h-3.5 w-3.5" />
          {/if}
          Full
        </Button>
      </div>
    </div>
    {#if scanError}
      <p class="rounded-lg border border-red-500/25 bg-red-500/10 px-3 py-2 text-sm text-red-300">
        {scanError}
      </p>
    {/if}
    {#if report}
      <HealthReportView {report} fullScan={fullScan} compact={true} />
    {/if}
  </section>

  <div class="flex items-center justify-between gap-3">
    <p class="text-xs font-medium uppercase tracking-wide text-white/40">History</p>
    <Button variant="outline" size="sm" disabled={loading} onclick={() => load(page)}>
      {#if loading}
        <Loader2 class="h-3.5 w-3.5 animate-spin" />
      {:else}
        <RefreshCw class="h-3.5 w-3.5" />
      {/if}
    </Button>
  </div>

  {#if error}
    <p class="rounded-lg border border-red-500/25 bg-red-500/10 px-3 py-2 text-sm text-red-300">
      {error}
    </p>
  {/if}

  {#if loading && scans.length === 0}
    <div class="flex justify-center py-12 text-white/40">
      <Loader2 class="h-6 w-6 animate-spin" />
    </div>
  {:else if scans.length === 0}
    <p class="text-sm text-muted-foreground">
      No scans recorded yet. Run a quick or full scan above, or wait for the scheduler.
    </p>
  {:else}
    <ul class="space-y-2">
      {#each scans as scan (scan.id)}
        <li>
          <button
            type="button"
            class="flex w-full items-center gap-3 rounded-xl border border-white/10 bg-card/60 px-4 py-3 text-left transition-colors hover:border-white/20"
            onclick={() => goto(`/settings/health-log/${scan.id}`)}
          >
            <ClipboardList class="h-4 w-4 shrink-0 text-white/40" />
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <span class="text-sm font-medium text-white">{formatWhen(scan.checked_at)}</span>
                <Badge class="capitalize border-current/30 bg-transparent {overallStatusClass(scan.status)}">
                  {scan.status}
                </Badge>
                {#if scan.full_scan}
                  <span class="text-xs text-white/40">full</span>
                {:else}
                  <span class="text-xs text-white/40">quick</span>
                {/if}
              </div>
              <p class="mt-0.5 text-xs text-white/45">
                {scan.duration_ms}ms
                {#if scan.fail_count > 0}
                  · {scan.fail_count} failed
                {/if}
                {#if scan.warn_count > 0}
                  · {scan.warn_count} warnings
                {/if}
              </p>
            </div>
            <ChevronRight class="h-4 w-4 shrink-0 text-white/30" />
          </button>
        </li>
      {/each}
    </ul>

    {#if totalPages > 1}
      <div class="flex items-center justify-center gap-3 pt-2">
        <Button
          variant="outline"
          size="sm"
          disabled={loading || page <= 1}
          onclick={() => load(page - 1)}
        >
          Previous
        </Button>
        <span class="text-xs text-white/45">Page {page} / {totalPages}</span>
        <Button
          variant="outline"
          size="sm"
          disabled={loading || page >= totalPages}
          onclick={() => load(page + 1)}
        >
          Next
        </Button>
      </div>
    {/if}
  {/if}
</div>
