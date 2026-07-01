<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { Button } from '$lib/components/ui/button';
  import HealthReportView from '$lib/components/health/health-report-view.svelte';
  import { getHealthScan } from '$lib/health/api';
  import type { ScanLogDetail } from '$lib/types/health-log';
  import { ApiError } from '$lib/api/client';
  import { ArrowLeft, Loader2 } from 'lucide-svelte';

  let detail = $state<ScanLogDetail | null>(null);
  let loading = $state(true);
  let error = $state('');

  const scanId = $derived(Number(page.params.id));

  async function load() {
    if (!scanId || Number.isNaN(scanId)) {
      error = 'Invalid scan id';
      loading = false;
      return;
    }
    loading = true;
    error = '';
    try {
      detail = await getHealthScan(scanId);
    } catch (e) {
      detail = null;
      error = e instanceof ApiError ? e.message : 'Failed to load scan';
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    void load();
  });
</script>

<div class="space-y-5 pb-8">
  <Button variant="outline" size="sm" onclick={() => goto('/settings/health-log')}>
    <ArrowLeft class="mr-1.5 h-3.5 w-3.5" />
    Scan log
  </Button>

  {#if loading}
    <div class="flex justify-center py-16 text-white/40">
      <Loader2 class="h-6 w-6 animate-spin" />
    </div>
  {:else if error}
    <p class="rounded-lg border border-red-500/25 bg-red-500/10 px-3 py-2 text-sm text-red-300">
      {error}
    </p>
  {:else if detail}
    <div>
      <h2 class="text-base font-semibold text-white">Scan #{detail.id}</h2>
      <p class="text-sm text-muted-foreground">
        {detail.duration_ms}ms · {detail.fail_count} failed · {detail.warn_count} warnings
      </p>
    </div>
    <HealthReportView report={detail.report} fullScan={detail.full_scan} />
  {/if}
</div>
