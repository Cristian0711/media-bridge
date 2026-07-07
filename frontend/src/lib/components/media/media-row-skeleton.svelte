<script lang="ts">
  import { Skeleton } from '$lib/components/ui/skeleton';

  // Mirrors the card geometry of media-library-item / request-card / media-card:
  // a 44×64 poster thumb, two text lines, and a trailing action stub.
  interface Props {
    count?: number;
  }

  let { count = 8 }: Props = $props();
</script>

{#each Array(count) as _, i (i)}
  <div class="mb-2 overflow-hidden rounded-lg border border-border/40 bg-card">
    <div class="flex items-center gap-3 p-3">
      <Skeleton class="h-16 w-11 shrink-0 rounded" />
      <div class="min-w-0 flex-1 space-y-2">
        <Skeleton class="h-3.5 w-2/3" />
        <Skeleton class="h-3 w-1/2" />
      </div>
      <div class="flex shrink-0 flex-col items-end gap-2">
        <Skeleton class="h-6 w-16 rounded-md" />
      </div>
    </div>
  </div>
{/each}
