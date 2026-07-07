import { writable } from 'svelte/store';

// `beforeinstallprompt` isn't in the standard DOM lib yet.
interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
}

let deferred: BeforeInstallPromptEvent | null = null;

/** True once the browser has offered an install prompt we can replay on demand. */
export const canInstall = writable(false);

/** Wire up capture of the deferred install prompt. Call once on app mount. */
export function initInstallPrompt(): void {
  if (typeof window === 'undefined') return;

  window.addEventListener('beforeinstallprompt', (e) => {
    // Suppress the browser's own mini-infobar so we can surface our own affordance.
    e.preventDefault();
    deferred = e as BeforeInstallPromptEvent;
    canInstall.set(true);
  });

  window.addEventListener('appinstalled', () => {
    deferred = null;
    canInstall.set(false);
  });
}

/** Replay the captured prompt. No-op if none is pending. */
export async function promptInstall(): Promise<void> {
  if (!deferred) return;
  await deferred.prompt();
  await deferred.userChoice;
  // A prompt can only be used once; drop it either way.
  deferred = null;
  canInstall.set(false);
}
