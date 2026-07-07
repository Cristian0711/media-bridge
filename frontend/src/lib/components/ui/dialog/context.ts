import { getContext, setContext } from 'svelte';

const KEY = Symbol('dialog');

export interface DialogContext {
  /** Close the dialog (wraps the owner's onOpenChange(false)). */
  close: () => void;
}

export function setDialogContext(ctx: DialogContext): void {
  setContext(KEY, ctx);
}

export function getDialogContext(): DialogContext | undefined {
  return getContext(KEY);
}
