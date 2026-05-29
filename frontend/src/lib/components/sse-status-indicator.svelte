<script lang="ts">
  import { sseConnectionStatus, type SseConnectionStatus } from '$lib/sse/connection-status';
  import { Radio, Wifi, WifiOff } from 'lucide-svelte';

  const status = $derived($sseConnectionStatus);

  const label: Record<SseConnectionStatus, string> = {
    connected: 'Live updates connected',
    connecting: 'Connecting to live updates…',
    disconnected: 'Live updates disconnected',
  };

  const iconClass = $derived(
    status === 'connected'
      ? 'text-emerald-400'
      : status === 'connecting'
        ? 'text-amber-400 animate-pulse'
        : 'text-white/35',
  );
</script>

<span
  class="sse-status-indicator inline-flex items-center justify-center {iconClass}"
  title={label[status]}
  aria-label={label[status]}
  role="status"
>
  {#if status === 'connected'}
    <Radio class="h-4 w-4" strokeWidth={2.25} />
  {:else if status === 'connecting'}
    <Wifi class="h-4 w-4" strokeWidth={2.25} />
  {:else}
    <WifiOff class="h-4 w-4" strokeWidth={2.25} />
  {/if}
</span>
