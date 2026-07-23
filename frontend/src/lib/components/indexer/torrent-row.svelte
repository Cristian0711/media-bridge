<script lang="ts">
  import type { Snippet } from 'svelte';
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { formatGbFixed } from '$lib/utils/format-size';
  import { Download, Users, ArrowDownToLine, HardDrive, Layers } from 'lucide-svelte';

  interface Props {
    name: string;
    indexerName: string;
    quality: string;
    category: string;
    freeleech: number;
    size: number;
    seeders: number;
    leechers: number;
    downloading: boolean;
    onDownload: () => void;
    /** Amber border variant used for unparsed results. */
    unparsed?: boolean;
    /** Distinct indexers carrying this release; a badge shows when > 1. */
    crossSeedCount?: number;
    /** Names of those indexers, listed under the cross-seed badge. */
    crossSeedIndexers?: string[];
    /** Extra badges rendered between the quality and category badges. */
    extraBadges?: Snippet;
  }

  let {
    name,
    indexerName,
    quality,
    category,
    freeleech,
    size,
    seeders,
    leechers,
    downloading,
    onDownload,
    unparsed = false,
    crossSeedCount = 1,
    crossSeedIndexers = [],
    extraBadges,
  }: Props = $props();
</script>

<div
  class="min-w-0 overflow-hidden rounded-lg border p-2.5 {unparsed
    ? 'border-amber-500/40 bg-card/50'
    : 'border-border/40 bg-card/50'}"
>
  <div class="mb-1.5 flex min-w-0 items-start justify-between gap-2">
    <h3 class="min-w-0 flex-1 break-all text-xs font-semibold">{name}</h3>
    <Button
      onclick={onDownload}
      size="sm"
      variant="ghost"
      class="h-6 w-6 shrink-0 p-0"
      disabled={downloading}
    >
      <Download class="h-3.5 w-3.5" />
    </Button>
  </div>
  <div class="mb-2 flex flex-wrap gap-1.5">
    <Badge variant="outline">{indexerName}</Badge>
    <Badge variant="secondary">{quality}</Badge>
    {#if extraBadges}{@render extraBadges()}{/if}
    <Badge variant="outline">{category}</Badge>
    {#if freeleech === 1}
      <Badge variant="default" class="bg-green-600">Freeleech</Badge>
    {/if}
  </div>
  {#if crossSeedCount > 1}
    <div class="mb-2 flex flex-wrap items-center gap-x-2 gap-y-1">
      <Badge
        variant="default"
        class="bg-sky-600"
        title="Found on {crossSeedCount} indexers — cross-seedable"
      >
        <Layers class="mr-1 h-3 w-3" />Cross-seed ×{crossSeedCount}
      </Badge>
      {#if crossSeedIndexers.length > 0}
        <span class="min-w-0 break-words text-[0.65rem] text-muted-foreground">
          {crossSeedIndexers.join(' · ')}
        </span>
      {/if}
    </div>
  {/if}
  <div class="flex flex-wrap gap-3 text-[0.65rem] text-muted-foreground">
    <span class="inline-flex items-center gap-1"><HardDrive class="h-3 w-3" />{formatGbFixed(size)}</span>
    <span class="inline-flex items-center gap-1 text-green-500"><ArrowDownToLine class="h-3 w-3" />{seeders}</span>
    <span class="inline-flex items-center gap-1 text-amber-500"><Users class="h-3 w-3" />{leechers}</span>
  </div>
</div>
