<script lang="ts">
  import { Skeleton } from '$lib/components/ui/skeleton';

  // Mirrors browse-row's horizontal card strip: 120×304 cards with a 180px-tall
  // poster, two title lines, and two action buttons.
  interface Props {
    count?: number;
  }

  let { count = 6 }: Props = $props();
</script>

<div class="-mx-1 flex gap-3 overflow-x-hidden px-1 pb-1">
  {#each Array(count) as _, i (i)}
    <div class="w-[7.5rem] shrink-0">
      <div class="flex h-[19rem] flex-col overflow-hidden rounded-lg border border-border/40 bg-card">
        <Skeleton class="h-[11.25rem] w-full rounded-none" />
        <div class="flex h-[7.75rem] flex-col gap-2 p-2">
          <Skeleton class="h-3 w-full" />
          <Skeleton class="h-3 w-2/3" />
          <div class="mt-auto flex flex-col gap-1">
            <Skeleton class="h-6 w-full rounded-md" />
            <Skeleton class="h-6 w-full rounded-md" />
          </div>
        </div>
      </div>
    </div>
  {/each}
</div>
