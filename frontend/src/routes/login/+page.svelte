<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import AuthForm from '$lib/components/auth/auth-form.svelte';
  import AuthField from '$lib/components/auth/auth-field.svelte';
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

<AuthForm
  title="Sign in"
  subtitle="Use your account to continue"
  {error}
  {loading}
  submitLabel="Sign in"
  loadingLabel="Signing in…"
  onSubmit={handleSubmit}
>
  <AuthField
    id="username"
    label="Username"
    autocomplete="username"
    required
    minlength={3}
    maxlength={32}
    bind:value={username}
  />
  <AuthField
    id="password"
    label="Password"
    type="password"
    autocomplete="current-password"
    required
    minlength={6}
    maxlength={32}
    bind:value={password}
  />

  {#snippet footer()}
    <p class="text-center text-sm text-muted-foreground">
      No account?
      <a href="/register" class="text-primary hover:underline">Register</a>
    </p>
  {/snippet}
</AuthForm>
