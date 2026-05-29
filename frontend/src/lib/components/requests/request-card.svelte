<script lang="ts">
  import { Badge } from '$lib/components/ui/badge';
  import PosterThumb from '$lib/components/media/poster-thumb.svelte';
  import {
    formatRelativeTime,
    isShowRequest,
    posterUrl,
    requestActionLabel,
    requestMediaKind,
    showScope,
    statusBadgeClass,
  } from '$lib/requests/map';
  import type { RequestRow } from '$lib/types/request';
  import { cn } from '$lib/utils';
  import RequestTorrentDialog from './request-torrent-dialog.svelte';
  import {
    CheckCircle2,
    Clock,
    Download,
    HardDrive,
    Loader2,
    Package,
    Plus,
    Trash2,
    XCircle,
  } from 'lucide-svelte';

  interface Props {
    request: RequestRow;
    showUsername?: boolean;
    index?: number;
  }

  let { request, showUsername = false, index = 99 }: Props = $props();

  let torrentOpen = $state(false);
  let torrentPreparing = $state(false);

  const imageSrc = $derived(posterUrl(request.poster_url));
  const isRemove = $derived(request.type.includes('remove'));
  const isDownload = $derived(request.type.includes('download'));
  const scope = $derived(isShowRequest(request.type) ? showScope(request) : '');
  const mediaKind = $derived(requestMediaKind(request.type));

  const actionChipClass =
    'inline-flex min-w-[6.5rem] w-[6.5rem] items-center justify-center gap-1 rounded-md border px-2 py-1 text-[10px] font-medium';
</script>

<div class="mb-2 overflow-hidden rounded-lg border border-border/40 bg-card transition-colors hover:bg-muted/20">
  <div class="flex items-start gap-3 p-3">
    <div class="shrink-0">
      <PosterThumb
        src={imageSrc}
        alt="{request.name} poster"
        priority={index < 8}
        fallback={request.type.startsWith('movie') ? 'movie' : 'show'}
      />
    </div>

    <div class="min-w-0 flex-1 space-y-1.5">
      <div class="flex min-w-0 items-center gap-2">
        <h4 class="min-w-0 truncate text-sm font-semibold leading-tight text-white">{request.name}</h4>
        {#if scope}
          <span class="shrink-0 rounded bg-white/10 px-1.5 py-0.5 text-[10px] font-medium text-white/75">
            {scope}
          </span>
        {/if}
      </div>

      <div class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-0.5 text-[11px] text-white/80">
        <span class="inline-flex shrink-0 items-center gap-1">
          {#if isRemove}
            <Trash2 class="h-3 w-3" />
          {:else}
            <Plus class="h-3 w-3" />
          {/if}
          {requestActionLabel(request.type)}
        </span>
        {#if mediaKind}
          <span class="text-white/50">·</span>
          <span class="shrink-0">{mediaKind}</span>
        {/if}
        <span class="text-white/50">·</span>
        <span class="shrink-0">{request.quality}</span>
        {#if showUsername}
          <span class="text-white/50">·</span>
          <span class="shrink-0">@{request.username}</span>
        {/if}
      </div>
    </div>

    <div class="flex shrink-0 flex-col items-end gap-1.5">
      <Badge class="{actionChipClass} {statusBadgeClass(request.status)}">
        {#if request.status === 'pending'}
          <Clock class="h-3 w-3 shrink-0" />
        {:else if request.status === 'queued'}
          <Clock class="h-3 w-3 shrink-0" />
        {:else if request.status === 'downloading'}
          <Download class="h-3 w-3 shrink-0" />
        {:else if request.status === 'downloaded'}
          <CheckCircle2 class="h-3 w-3 shrink-0" />
        {:else if request.status === 'removing'}
          <Loader2 class="h-3 w-3 shrink-0 animate-spin" />
        {:else if request.status === 'removed' || request.status === 'processed'}
          <CheckCircle2 class="h-3 w-3 shrink-0" />
        {:else if request.status === 'cancelled'}
          <Package class="h-3 w-3 shrink-0" />
        {:else if request.status === 'failed'}
          <XCircle class="h-3 w-3 shrink-0" />
        {:else}
          <Loader2 class="h-3 w-3 shrink-0" />
        {/if}
        {request.status}
      </Badge>

      {#if isDownload}
        <button
          type="button"
          class={cn(
            actionChipClass,
            'border-white/15 bg-white/5 text-white/85 transition-colors hover:bg-white/10',
            torrentPreparing && 'pointer-events-none opacity-70',
          )}
          disabled={torrentPreparing}
          onclick={() => (torrentOpen = true)}
        >
          {#if torrentPreparing}
            <Loader2 class="h-3 w-3 shrink-0 animate-spin" />
          {:else}
            <HardDrive class="h-3 w-3 shrink-0" />
          {/if}
          Torrent
        </button>
        <span class="w-[6.5rem] text-center text-[10px] leading-tight text-white/65">
          {formatRelativeTime(request.created_at)}
        </span>
      {:else}
        <span class="text-[10px] text-white/65">{formatRelativeTime(request.created_at)}</span>
      {/if}
    </div>
  </div>
</div>

{#if isDownload}
  <RequestTorrentDialog
    open={torrentOpen}
    onOpenChange={(v) => (torrentOpen = v)}
    onPreparingChange={(v) => (torrentPreparing = v)}
    {request}
  />
{/if}
