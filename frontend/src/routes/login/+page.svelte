<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { Button } from '$lib/components/ui/button';
  import AuthFormCard from '$lib/components/auth/auth-form-card.svelte';
  import { login } from '$lib/auth/api';
  import { ApiError } from '$lib/api/client';

  let username = $state('');
  let password = $state('');
  let error = $state('');
  let loading = $state(false);

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    loading = true;

    try {
      await login({ username: username.trim(), password });
      const redirect = page.url.searchParams.get('redirect');
      goto(redirect ? decodeURIComponent(redirect) : '/');
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Login failed';
    } finally {
      loading = false;
    }
  }
</script>

<div
  class="flex min-h-0 flex-1 items-center justify-center overflow-hidden px-6 pb-[max(1.25rem,env(safe-area-inset-bottom))]"
>
  <AuthFormCard title="Sign in" subtitle="Use your account to continue">
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
          autocomplete="current-password"
          required
          minlength="6"
          maxlength="32"
          bind:value={password}
          class="auth-input"
        />
      </div>

      {#if error}
        <p class="text-sm text-red-400">{error}</p>
      {/if}

      <Button type="submit" class="w-full" disabled={loading}>
        {loading ? 'Signing in…' : 'Sign in'}
      </Button>
    </form>

    {#snippet footer()}
      <p class="text-center text-sm text-muted-foreground">
        No account?
        <a href="/register" class="text-primary hover:underline">Register</a>
      </p>
    {/snippet}
  </AuthFormCard>
</div>
