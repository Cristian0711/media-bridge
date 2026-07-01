<script lang="ts">
  import { Badge } from '$lib/components/ui/badge';
  import {
    checkStatusClass,
    overallStatusClass,
    overallStatusLabel,
  } from '$lib/health/status';
  import type { HealthCheck, HealthReport, LinkIssueSample } from '$lib/types/health';
  import {
    AlertTriangle,
    CheckCircle2,
    Circle,
    MinusCircle,
    XCircle,
  } from 'lucide-svelte';

  interface Props {
    report: HealthReport;
    fullScan?: boolean;
    compact?: boolean;
  }

  let { report, fullScan = false, compact = false }: Props = $props();

  function checkIcon(status: string) {
    switch (status) {
      case 'ok':
        return CheckCircle2;
      case 'warn':
        return AlertTriangle;
      case 'fail':
        return XCircle;
      case 'skip':
        return MinusCircle;
      default:
        return Circle;
    }
  }

  function issuesFromCheck(c: HealthCheck): LinkIssueSample[] {
    const sample = c.details?.issues_sample;
    if (!Array.isArray(sample)) return [];
    return sample as LinkIssueSample[];
  }

  const fsChecks = $derived(report.checks.filter((c) => c.id.startsWith('fs_')));
  const coreChecks = $derived(report.checks.filter((c) => !c.id.startsWith('fs_')));
</script>

<div class="space-y-4">
  <div
    class="flex items-center gap-3 rounded-xl border px-4 py-3 {overallStatusClass(report.status)}"
  >
    <div class="min-w-0 flex-1">
      <p class="font-medium capitalize">{overallStatusLabel(report.status)}</p>
      <p class="text-xs opacity-80">
        {new Date(report.checked_at).toLocaleString()}
        {#if fullScan}
          · full scan
        {:else}
          · quick scan
        {/if}
      </p>
    </div>
    <Badge class="capitalize border-current/30 bg-transparent">{report.status}</Badge>
  </div>

  <ul class="space-y-2">
    {#each coreChecks as check (check.id)}
      {@const Icon = checkIcon(check.status)}
      <li class="rounded-xl border border-white/10 bg-card/60 px-4 py-3">
        <div class="flex items-start gap-3">
          <Icon class="mt-0.5 h-4 w-4 shrink-0 {checkStatusClass(check.status)}" />
          <div class="min-w-0 flex-1">
            <p class="text-sm font-medium text-white">{check.name}</p>
            <p class="mt-0.5 text-sm text-white/55">{check.message}</p>
            {#if !compact && check.id === 'request_pipeline' && check.details?.by_status}
              <div class="mt-2 flex flex-wrap gap-1.5">
                {#each Object.entries(check.details.by_status as Record<string, number>) as [status, count]}
                  <span
                    class="rounded-md border border-white/10 bg-white/5 px-2 py-0.5 text-xs text-white/60"
                  >
                    {status}: {count}
                  </span>
                {/each}
              </div>
            {/if}
            {#if !compact && check.id === 'media_torrent_registry'}
              {#if Array.isArray(check.details?.media_issues) && (check.details.media_issues as unknown[]).length > 0}
                <ul class="mt-2 max-h-32 space-y-1 overflow-y-auto text-xs text-red-300/90">
                  {#each check.details.media_issues as issue}
                    <li class="truncate font-mono" title={issue.message}>
                      media #{issue.media_id} · {issue.message}
                    </li>
                  {/each}
                </ul>
              {/if}
              {#if Array.isArray(check.details?.orphan_torrents) && (check.details.orphan_torrents as unknown[]).length > 0}
                <ul class="mt-2 max-h-32 space-y-1 overflow-y-auto text-xs text-amber-300/90">
                  {#each check.details.orphan_torrents as issue}
                    <li class="truncate font-mono" title={issue.message}>
                      {issue.hash?.slice(0, 12)}… · {issue.torrent_name ?? issue.message}
                    </li>
                  {/each}
                </ul>
              {/if}
            {/if}
          </div>
        </div>
      </li>
    {/each}
  </ul>

  {#if fsChecks.length > 0}
    <ul class="space-y-2">
      {#each fsChecks as check (check.id)}
        {@const Icon = checkIcon(check.status)}
        {@const issues = issuesFromCheck(check)}
        <li class="rounded-xl border border-white/10 bg-card/60 px-4 py-3">
          <div class="flex items-start gap-3">
            <Icon class="mt-0.5 h-4 w-4 shrink-0 {checkStatusClass(check.status)}" />
            <div class="min-w-0 flex-1">
              <p class="text-sm font-medium text-white">{check.name}</p>
              <p class="mt-0.5 text-sm text-white/55">{check.message}</p>
              {#if issues.length > 0 && !compact}
                <ul class="mt-2 max-h-32 space-y-1 overflow-y-auto text-xs text-red-300/90">
                  {#each issues as issue}
                    <li class="truncate font-mono" title={issue.path}>
                      nlink={issue.nlink} · {issue.path}
                    </li>
                  {/each}
                </ul>
              {/if}
            </div>
          </div>
        </li>
      {/each}
    </ul>
  {/if}
</div>
