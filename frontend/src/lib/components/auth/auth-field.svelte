<script lang="ts">
  import type { HTMLInputAttributes } from 'svelte/elements';

  interface Props {
    id: string;
    label: string;
    value: string;
    type?: HTMLInputAttributes['type'];
    name?: string;
    autocomplete?: HTMLInputAttributes['autocomplete'];
    required?: boolean;
    minlength?: number;
    maxlength?: number;
    placeholder?: string;
  }

  let {
    id,
    label,
    value = $bindable(),
    type = 'text',
    name = id,
    autocomplete,
    required = false,
    minlength,
    maxlength,
    placeholder,
  }: Props = $props();
</script>

<div class="space-y-2">
  <label for={id} class="text-sm font-medium text-white/80">{label}</label>
  <input
    {id}
    {name}
    {type}
    {autocomplete}
    {required}
    {minlength}
    {maxlength}
    {placeholder}
    {value}
    oninput={(e) => (value = e.currentTarget.value)}
    class="auth-input"
  />
</div>
