<script lang="ts">
  import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
  } from '$lib/components/ui/dialog';
  import { Button } from '$lib/components/ui/button';
  import { getRequestTorrentInfo } from '$lib/requests/torrent-api';
  import {
    formatBytes,
    formatEta,
    formatProgress,
    formatSpeed,
    stateLabel,
  } from '$lib/requests/torrent-format';
  import type { RequestRow } from '$lib/types/request';
  import type { RequestTorrentInfo } from '$lib/types/torrent';
  import { CheckCircle2, Circle, HardDrive, Link2, Loader2, RefreshCw } from 'lucide-svelte';
  import { untrack } from 'svelte';

  const POLL_MS = 5000;

  interface Props {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    request: RequestRow;
    /** Fired while the initial fetch runs before the dialog is shown. */
    onPreparingChange?: (preparing: boolean) => void;
  }

  let { open, onOpenChange, request, onPreparingChange }: Props = $props();

  let info = $state<RequestTorrentInfo | null>(null);
  let loading = $state(false);
  let error = $state<string | null>(null);
  let lastUpdated = $state<Date | null>(null);
  let refreshInFlight = false;
  /** Dialog stays closed until the first fetch for this open cycle completes. */
  let dialogVisible = $state(false);

  const displayName = $derived(info?.torrent?.name || info?.torrent_name || request.torrent_name || request.name);
  const progressPct = $derived(
    info?.torrent ? Math.min(100, Math.max(0, info.torrent.progress * 100)) : 0,
  );
  const hardlinkPct = $derived(
    info?.hardlink && info.hardlink.total > 0
      ? Math.min(100, (info.hardlink.linked / info.hardlink.total) * 100)
      : 0,
  );

  $effect(() => {
    onPreparingChange?.(open && !dialogVisible);
  });

  async function refresh() {
    if (refreshInFlight) return;
    refreshInFlight = true;
    const showSpinner = untrack(() => info === null);
    if (showSpinner) loading = true;
    error = null;
    try {
      info = await getRequestTorrentInfo(request.id);
      lastUpdated = new Date();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load torrent info';
    } finally {
      refreshInFlight = false;
      if (showSpinner) loading = false;
    }
  }

  function pollVisible() {
    if (typeof document !== 'undefined' && document.visibilityState === 'hidden') return;
    void refresh();
  }

  function resetState() {
    info = null;
    error = null;
    lastUpdated = null;
    loading = false;
    refreshInFlight = false;
    dialogVisible = false;
  }

  function handleDialogOpenChange(v: boolean) {
    if (!v) {
      onOpenChange(false);
    }
  }

  $effect(() => {
    if (!open) {
      resetState();
      return;
    }

    const requestId = request.id;
    let cancelled = false;

    void (async () => {
      await refresh();
      if (!cancelled && open && request.id === requestId) {
        dialogVisible = true;
      }
    })();

    return () => {
      cancelled = true;
    };
  });

  $effect(() => {
    if (!dialogVisible) return;

    const requestId = request.id;
    const intervalId = setInterval(pollVisible, POLL_MS);
    const onVisibility = () => {
      if (document.visibilityState === 'visible') pollVisible();
    };
    document.addEventListener('visibilitychange', onVisibility);

    return () => {
      clearInterval(intervalId);
      document.removeEventListener('visibilitychange', onVisibility);
    };
  });
</script>

<Dialog open={dialogVisible} onOpenChange={handleDialogOpenChange}>
  <DialogContent class="max-w-lg gap-0 overflow-hidden p-0">
    <DialogHeader class="space-y-1.5 border-b border-white/10 px-5 pb-4 pt-5">
      <DialogTitle>Torrent</DialogTitle>
      <DialogDescription class="line-clamp-2 text-white/60">{displayName}</DialogDescription>
    </DialogHeader>

    <div class="min-h-0 max-h-[min(60vh,28rem)] overflow-y-auto px-5 py-5">
      {#if error && !info}
        <p class="rounded-xl border border-red-500/25 bg-red-500/10 px-4 py-8 text-center text-sm text-red-300">
          {error}
        </p>
      {:else if info}
        <div class="space-y-4">
          {#if info.message && !info.torrent}
            <p class="rounded-xl border border-white/10 bg-white/[0.04] px-4 py-4 text-sm leading-relaxed text-white/75">
              {info.message}
            </p>
          {/if}

          {#if info.torrent}
            <section class="rounded-xl border border-white/10 bg-white/[0.04] p-4">
              <div class="mb-3 flex items-center justify-between gap-3 text-xs">
                <span class="font-medium text-white/55">Download progress</span>
                <span class="tabular-nums font-semibold text-white">{formatProgress(info.torrent.progress)}</span>
              </div>
              <div class="h-2.5 overflow-hidden rounded-full bg-white/10">
                <div
                  class="h-full rounded-full bg-gradient-to-r from-green-600/90 to-green-400/90 transition-[width] duration-500"
                  style:width="{progressPct}%"
                ></div>
              </div>
            </section>

            <section class="rounded-xl border border-white/10 bg-white/[0.04] p-4">
              <h3 class="mb-3 text-xs font-semibold uppercase tracking-wide text-white/45">Transfer</h3>
              <dl class="grid grid-cols-2 gap-x-6 gap-y-3 text-sm">
                <div>
                  <dt class="text-[11px] text-white/45">State</dt>
                  <dd class="mt-0.5 font-medium text-white/90">{stateLabel(info.torrent.state)}</dd>
                </div>
                <div>
                  <dt class="text-[11px] text-white/45">ETA</dt>
                  <dd class="mt-0.5 font-medium tabular-nums text-white/90">{formatEta(info.torrent.eta)}</dd>
                </div>
                <div>
                  <dt class="text-[11px] text-white/45">Download</dt>
                  <dd class="mt-0.5 font-medium tabular-nums text-white/90">{formatSpeed(info.torrent.dlspeed)}</dd>
                </div>
                <div>
                  <dt class="text-[11px] text-white/45">Upload</dt>
                  <dd class="mt-0.5 font-medium tabular-nums text-white/90">{formatSpeed(info.torrent.upspeed)}</dd>
                </div>
                <div>
                  <dt class="text-[11px] text-white/45">Size</dt>
                  <dd class="mt-0.5 font-medium tabular-nums text-white/90">{formatBytes(info.torrent.size)}</dd>
                </div>
                <div>
                  <dt class="text-[11px] text-white/45">Downloaded</dt>
                  <dd class="mt-0.5 font-medium tabular-nums text-white/90">{formatBytes(info.torrent.downloaded)}</dd>
                </div>
                <div class="col-span-2">
                  <dt class="text-[11px] text-white/45">Peers</dt>
                  <dd class="mt-0.5 font-medium text-white/90">
                    {info.torrent.seeders} seeders · {info.torrent.leechers} leechers
                  </dd>
                </div>
              </dl>
            </section>
          {/if}

          <div class="flex flex-wrap gap-2">
            {#if info.hardlink}
              <span
                class="inline-flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-xs font-medium {info.hardlink.complete
                  ? 'border-green-500/30 bg-green-500/10 text-green-400'
                  : 'border-yellow-500/30 bg-yellow-500/10 text-yellow-400'}"
              >
                <Link2 class="h-3.5 w-3.5" />
                Hardlink {info.hardlink.complete ? 'ready' : `${info.hardlink.linked}/${info.hardlink.total}`}
              </span>
            {/if}
            {#if info.torrent_hash}
              <span
                class="inline-flex max-w-full items-center gap-1.5 rounded-full border border-white/10 bg-white/5 px-3 py-1.5 font-mono text-[11px] text-white/65"
              >
                <HardDrive class="h-3.5 w-3.5 shrink-0" />
                {info.torrent_hash.slice(0, 16)}…
              </span>
            {/if}
          </div>

          {#if info.hardlink && info.hardlink.total > 0}
            <section class="rounded-xl border border-white/10 bg-white/[0.04] p-4">
              <div class="mb-3 flex items-center justify-between gap-3">
                <h3 class="text-xs font-semibold uppercase tracking-wide text-white/45">Library hardlinks</h3>
                <span class="tabular-nums text-xs font-semibold text-white">
                  {info.hardlink.linked}/{info.hardlink.total}
                </span>
              </div>
              <div class="mb-4 h-2 overflow-hidden rounded-full bg-white/10">
                <div
                  class="h-full rounded-full bg-gradient-to-r from-sky-600/90 to-sky-400/90 transition-[width] duration-500"
                  style:width="{hardlinkPct}%"
                ></div>
              </div>

              {#if info.hardlink.done.length > 0}
                <div class="mb-3">
                  <p class="mb-2 text-[11px] font-medium text-green-400/90">
                    Done ({info.hardlink.done.length})
                  </p>
                  <ul class="max-h-28 space-y-1 overflow-y-auto rounded-lg border border-green-500/15 bg-green-500/5 p-2">
                    {#each info.hardlink.done as file (`done:${file.name}:${file.size}`)}
                      <li class="flex items-start gap-2 text-xs text-white/85">
                        <CheckCircle2 class="mt-0.5 h-3.5 w-3.5 shrink-0 text-green-400" />
                        <span class="min-w-0 flex-1 break-all leading-snug">{file.name}</span>
                        <span class="shrink-0 tabular-nums text-white/45">{formatBytes(file.size)}</span>
                      </li>
                    {/each}
                  </ul>
                </div>
              {/if}

              {#if info.hardlink.remaining.length > 0}
                <div class="mb-3">
                  <p class="mb-2 text-[11px] font-medium text-amber-400/90">
                    Remaining ({info.hardlink.remaining.length})
                  </p>
                  <ul class="max-h-28 space-y-1 overflow-y-auto rounded-lg border border-amber-500/15 bg-amber-500/5 p-2">
                    {#each info.hardlink.remaining as file (`rem:${file.name}:${file.size}`)}
                      <li class="flex items-start gap-2 text-xs text-white/75">
                        <Circle class="mt-0.5 h-3.5 w-3.5 shrink-0 text-amber-400/80" />
                        <span class="min-w-0 flex-1 break-all leading-snug">{file.name}</span>
                        <span class="shrink-0 tabular-nums text-white/45">{formatBytes(file.size)}</span>
                      </li>
                    {/each}
                  </ul>
                </div>
              {/if}

            </section>
          {:else if info.hardlink}
            <p class="rounded-xl border border-white/10 bg-white/[0.04] px-4 py-3 text-sm text-white/60">
              No torrent files listed yet — hardlink progress will appear once qBittorrent reports files.
            </p>
          {/if}

          {#if error}
            <p class="rounded-lg border border-red-500/20 bg-red-500/10 px-3 py-2 text-xs text-red-300">{error}</p>
          {/if}
        </div>
      {/if}
    </div>

    <DialogFooter class="border-t border-white/10 bg-white/[0.02] px-5 py-4">
      <span class="mr-auto self-center text-[11px] text-white/50">
        {#if lastUpdated}
          Updated {lastUpdated.toLocaleTimeString()}
        {:else}
          Auto-refreshes every {POLL_MS / 1000}s
        {/if}
      </span>
      <Button variant="outline" size="sm" onclick={() => void refresh()} disabled={loading}>
        <RefreshCw class="mr-1 h-3.5 w-3.5 {loading ? 'animate-spin' : ''}" />
        Refresh
      </Button>
      <Button variant="outline" size="sm" onclick={() => onOpenChange(false)}>Close</Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
