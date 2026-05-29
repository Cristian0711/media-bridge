<script lang="ts">
  import { goto } from '$app/navigation';
  import { Button } from '$lib/components/ui/button';
  import AuthFormCard from '$lib/components/auth/auth-form-card.svelte';
  import { register, login } from '$lib/auth/api';
  import { ApiError } from '$lib/api/client';

  let username = $state('');
  let password = $state('');
  let key = $state('');
  let error = $state('');
  let loading = $state(false);

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    loading = true;

    try {
      await register({
        username: username.trim(),
        password,
        key: key.trim(),
      });
      await login({ username: username.trim(), password });
      goto('/');
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Registration failed';
    } finally {
      loading = false;
    }
  }
</script>

<div
  class="flex min-h-0 flex-1 items-center justify-center overflow-hidden px-6 pb-[max(1.25rem,env(safe-area-inset-bottom))]"
>
  <AuthFormCard title="Create account" subtitle="You need an invite key to register">
    <form class="space-y-4" onsubmit={handleSubmit}>
      <div class="space-y-2">
        <label for="username" class="text-sm font-medium text-white/80">Username</label>
        <input
          id="username"
          name="username"
          type="text"
          autocomplete="username"
          required
          minlength="3"
          maxlength="32"
          bind:value={username}
          class="auth-input"
        />
      </div>

      <div class="space-y-2">
        <label for="password" class="text-sm font-medium text-white/80">Password</label>
        <input
          id="password"
          name="password"
          type="password"
          autocomplete="new-password"
          required
          minlength="6"
          maxlength="32"
          bind:value={password}
          class="auth-input"
        />
      </div>

      <div class="space-y-2">
        <label for="key" class="text-sm font-medium text-white/80">Invite key</label>
        <input
          id="key"
          name="key"
          type="text"
          required
          bind:value={key}
          class="auth-input"
          placeholder="Paste your invite key"
        />
      </div>

      {#if error}
        <p class="text-sm text-red-400">{error}</p>
      {/if}

      <Button type="submit" class="w-full" disabled={loading}>
        {loading ? 'Creating account…' : 'Register'}
      </Button>
    </form>

    {#snippet footer()}
      <p class="text-center text-sm text-muted-foreground">
        Already have an account?
        <a href="/login" class="text-primary hover:underline">Sign in</a>
      </p>
    {/snippet}
  </AuthFormCard>
</div>
