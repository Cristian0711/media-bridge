<script lang="ts">
  import SseStatusIndicator from '$lib/components/sse-status-indicator.svelte';
</script>

<header class="app-header">
  <div class="app-header-inner">
    <div class="app-header-brand">
      <img src="/icon-192.png" alt="" class="app-header-logo" width="32" height="32" />
      <span class="app-header-title">Media Bridge</span>
    </div>
    <SseStatusIndicator />
  </div>
</header>
