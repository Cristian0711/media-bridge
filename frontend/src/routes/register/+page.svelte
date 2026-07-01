<script lang="ts">
  import { goto } from '$app/navigation';
  import AuthForm from '$lib/components/auth/auth-form.svelte';
  import AuthField from '$lib/components/auth/auth-field.svelte';
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

<AuthForm
  title="Create account"
  subtitle="You need an invite key to register"
  {error}
  {loading}
  submitLabel="Register"
  loadingLabel="Creating account…"
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
    autocomplete="new-password"
    required
    minlength={6}
    maxlength={32}
    bind:value={password}
  />
  <AuthField
    id="key"
    label="Invite key"
    required
    placeholder="Paste your invite key"
    bind:value={key}
  />

  {#snippet footer()}
    <p class="text-center text-sm text-muted-foreground">
      Already have an account?
      <a href="/login" class="text-primary hover:underline">Sign in</a>
    </p>
  {/snippet}
</AuthForm>
