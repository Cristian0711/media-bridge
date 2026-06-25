<script lang="ts">
  import { Button } from '$lib/components/ui/button';
  import { Badge } from '$lib/components/ui/badge';
  import { generateRegistrationKey, listRegistrationKeys } from '$lib/auth/api';
  import type { InviteKey } from '$lib/types/auth';
  import { ClipboardList, Copy, KeyRound, Loader2 } from 'lucide-svelte';

  let generatedKey = $state<string | null>(null);
  let generatingKey = $state(false);
  let keyError = $state<string | null>(null);
  let copiedKey = $state(false);
  let showInviteKeys = $state(false);
  let inviteKeys = $state<InviteKey[]>([]);
  let loadingInviteKeys = $state(false);
  let inviteKeysError = $state<string | null>(null);

  async function loadInviteKeys() {
    loadingInviteKeys = true;
    inviteKeysError = null;
    try {
      const resp = await listRegistrationKeys();
      inviteKeys = resp.keys;
    } catch (e) {
      inviteKeys = [];
      inviteKeysError = e instanceof Error ? e.message : 'Failed to load invite keys';
    } finally {
      loadingInviteKeys = false;
    }
  }

  async function toggleInviteKeys() {
    showInviteKeys = !showInviteKeys;
    if (showInviteKeys) {
      await loadInviteKeys();
    }
  }

  async function createRegistrationKey() {
    generatingKey = true;
    keyError = null;
    copiedKey = false;
    try {
      const resp = await generateRegistrationKey();
      generatedKey = resp.key;
      if (showInviteKeys) {
        await loadInviteKeys();
      }
    } catch (e) {
      generatedKey = null;
      keyError = e instanceof Error ? e.message : 'Failed to generate key';
    } finally {
      generatingKey = false;
    }
  }

  function inviteKeyStatusLabel(status: InviteKey['status']) {
    return status === 'available' ? 'Available' : 'Used';
  }

  function inviteKeyStatusClass(status: InviteKey['status']) {
    return status === 'available'
      ? 'border-emerald-500/30 text-emerald-300'
      : 'border-white/20 text-white/50';
  }

  async function copyRegistrationKey() {
    if (!generatedKey) return;
    try {
      await navigator.clipboard.writeText(generatedKey);
      copiedKey = true;
    } catch {
      copiedKey = false;
    }
  }
</script>

<section class="space-y-3 border-t border-white/10 pt-6">
  <h2 class="text-base font-semibold text-white">Invite keys</h2>
  <p class="text-sm text-muted-foreground">
    Generate a one-time registration key for a new user. Each key works once.
  </p>
  <div class="flex flex-col gap-2 sm:flex-row">
    <Button variant="outline" disabled={generatingKey} onclick={createRegistrationKey}>
      {#if generatingKey}
        <Loader2 class="mr-2 h-4 w-4 animate-spin" />
        Generating…
      {:else}
        <KeyRound class="mr-2 h-4 w-4" />
        Generate registration key
      {/if}
    </Button>
    <Button variant="outline" onclick={toggleInviteKeys}>
      <ClipboardList class="mr-2 h-4 w-4" />
      {showInviteKeys ? 'Hide invite keys' : 'View all invite keys'}
    </Button>
  </div>
  {#if showInviteKeys}
    {#if loadingInviteKeys}
      <div class="flex justify-center py-4 text-white/40">
        <Loader2 class="h-5 w-5 animate-spin" />
      </div>
    {:else if inviteKeysError}
      <p class="text-sm text-red-400">{inviteKeysError}</p>
    {:else if inviteKeys.length === 0}
      <p class="text-sm text-muted-foreground">No invite keys yet.</p>
    {:else}
      <ul class="space-y-2">
        {#each inviteKeys as item (item.value)}
          <li class="rounded-xl border border-white/10 bg-card/60 px-4 py-3">
            <div class="flex items-start justify-between gap-3">
              <p class="min-w-0 flex-1 break-all font-mono text-sm text-white">{item.value}</p>
              <Badge class="shrink-0 capitalize border bg-transparent {inviteKeyStatusClass(item.status)}">
                {inviteKeyStatusLabel(item.status)}
              </Badge>
            </div>
            <p class="mt-2 text-xs text-white/45">
              Created {new Date(item.created_at).toLocaleString()}
              {#if item.used_at}
                · Used {new Date(item.used_at).toLocaleString()}
              {/if}
            </p>
          </li>
        {/each}
      </ul>
    {/if}
  {/if}
  {#if keyError}
    <p class="text-sm text-red-400">{keyError}</p>
  {/if}
  {#if generatedKey}
    <div class="rounded-xl border border-white/10 bg-card/60 px-4 py-3">
      <p class="text-xs text-white/45">New registration key</p>
      <p class="mt-1 break-all font-mono text-sm text-white">{generatedKey}</p>
      <Button variant="ghost" size="sm" class="mt-2" onclick={copyRegistrationKey}>
        <Copy class="mr-2 h-4 w-4" />
        {copiedKey ? 'Copied' : 'Copy key'}
      </Button>
    </div>
  {/if}
</section>
