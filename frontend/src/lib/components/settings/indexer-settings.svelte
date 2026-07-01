<script lang="ts">
  import { onMount } from 'svelte';
  import { Badge } from '$lib/components/ui/badge';
  import { listIndexerSettings, updateIndexerSetting } from '$lib/indexer/api';
  import type { IndexerSetting } from '$lib/types/indexer';
  import { Loader2, Server } from 'lucide-svelte';

  let indexers = $state<IndexerSetting[]>([]);
  let loading = $state(true);
  let loadError = $state<string | null>(null);
  let savingName = $state<string | null>(null);
  let saveError = $state<string | null>(null);

  async function load() {
    loading = true;
    loadError = null;
    try {
      const resp = await listIndexerSettings();
      indexers = resp.indexers;
    } catch (e) {
      indexers = [];
      loadError = e instanceof Error ? e.message : 'Failed to load indexers';
    } finally {
      loading = false;
    }
  }

  async function setFreeleechOnly(indexer: IndexerSetting, freeleechOnly: boolean) {
    if (indexer.freeleech_only === freeleechOnly || savingName) return;
    savingName = indexer.name;
    saveError = null;
    const previous = indexer.freeleech_only;
    // Optimistic update; revert on failure.
    indexers = indexers.map((i) =>
      i.name === indexer.name ? { ...i, freeleech_only: freeleechOnly } : i,
    );
    try {
      await updateIndexerSetting(indexer.name, freeleechOnly);
    } catch (e) {
      indexers = indexers.map((i) =>
        i.name === indexer.name ? { ...i, freeleech_only: previous } : i,
      );
      saveError = e instanceof Error ? e.message : 'Failed to save setting';
    } finally {
      savingName = null;
    }
  }

  onMount(load);
</script>

<section class="space-y-3 border-t border-white/10 pt-6">
  <h2 class="text-base font-semibold text-white">Indexers</h2>
  <p class="text-sm text-muted-foreground">
    Choose which results each indexer contributes. <span class="font-medium">Freeleech only</span>
    hides torrents that count against your ratio; <span class="font-medium">Include all</span>
    shows every result.
  </p>

  {#if loading}
    <div class="flex justify-center py-6 text-white/40">
      <Loader2 class="h-5 w-5 animate-spin" />
    </div>
  {:else if loadError}
    <p class="text-sm text-red-400">{loadError}</p>
  {:else if indexers.length === 0}
    <p class="text-sm text-muted-foreground">No indexers available.</p>
  {:else}
    {#if saveError}
      <p class="text-sm text-red-400">{saveError}</p>
    {/if}
    <ul class="space-y-2">
      {#each indexers as indexer (indexer.name)}
        <li class="rounded-xl border border-white/10 bg-card/60 px-4 py-3">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div class="flex min-w-0 items-center gap-3">
              <Server class="h-5 w-5 shrink-0 text-white/50" />
              <div class="min-w-0">
                <p class="truncate text-sm font-medium text-white">{indexer.name}</p>
                {#if !indexer.enabled}
                  <p class="text-xs text-white/40">Disabled in Prowlarr</p>
                {/if}
              </div>
              {#if savingName === indexer.name}
                <Loader2 class="h-4 w-4 shrink-0 animate-spin text-white/40" />
              {/if}
            </div>

            <div class="inline-flex shrink-0 overflow-hidden rounded-lg border border-white/10">
              <button
                type="button"
                disabled={savingName !== null}
                class="px-3 py-1.5 text-xs font-medium transition-colors disabled:opacity-60 {indexer.freeleech_only
                  ? 'bg-white/15 text-white'
                  : 'text-white/50 hover:text-white/80'}"
                onclick={() => setFreeleechOnly(indexer, true)}
              >
                Freeleech only
              </button>
              <button
                type="button"
                disabled={savingName !== null}
                class="border-l border-white/10 px-3 py-1.5 text-xs font-medium transition-colors disabled:opacity-60 {!indexer.freeleech_only
                  ? 'bg-white/15 text-white'
                  : 'text-white/50 hover:text-white/80'}"
                onclick={() => setFreeleechOnly(indexer, false)}
              >
                Include all
              </button>
            </div>
          </div>
          {#if !indexer.freeleech_only}
            <Badge class="mt-2 border-amber-500/30 bg-transparent text-amber-300">
              Non-freeleech results shown
            </Badge>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</section>
