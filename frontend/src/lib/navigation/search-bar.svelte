<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { Search } from 'lucide-svelte';
  import { cn } from '$lib/utils';
  import { setSearchInputFocused } from './search-ui.svelte';

  let query = $state('');

  $effect(() => {
    if (page.url.pathname.startsWith('/search')) {
      query = page.url.searchParams.get('q') ?? '';
    }
  });

  function submit() {
    const q = query.trim();
    if (!q) return;
    goto(`/search?q=${encodeURIComponent(q)}`);
  }

  function onKeydown(event: KeyboardEvent) {
    if (event.key === 'Enter') {
      event.preventDefault();
      submit();
    }
  }

  let inputEl = $state<HTMLInputElement | undefined>();

  function onFocus() {
    setSearchInputFocused(true);
    // iOS: keyboard + tab-bar hide animate async; nudge input into view after layout settles.
    requestAnimationFrame(() => {
      inputEl?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
      requestAnimationFrame(() => {
        inputEl?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
      });
    });
    setTimeout(() => inputEl?.scrollIntoView({ block: 'nearest', inline: 'nearest' }), 120);
    setTimeout(() => inputEl?.scrollIntoView({ block: 'nearest', inline: 'nearest' }), 300);
  }
</script>

<label
  class={cn(
    'flex w-full max-w-md items-center gap-3 rounded-full px-4 py-3',
    'border border-white/10 bg-white/[0.08] backdrop-blur-2xl backdrop-saturate-150',
    'shadow-[0_18px_40px_-12px_rgba(0,0,0,0.55),inset_0_1px_0_rgba(255,255,255,0.08)]',
  )}
>
  <Search class="size-5 shrink-0 text-white/55" strokeWidth={1.75} />
  <input
    bind:this={inputEl}
    type="search"
    name="query"
    placeholder="Search movies, shows…"
    autocomplete="off"
    enterkeyhint="search"
    bind:value={query}
    class="min-w-0 flex-1 bg-transparent text-base text-white outline-none placeholder:text-white/45"
    onfocus={onFocus}
    onblur={() => setSearchInputFocused(false)}
    onkeydown={onKeydown}
  />
</label>
