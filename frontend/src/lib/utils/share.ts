import { haptic } from './haptics';

interface ShareData {
  title: string;
  text?: string;
  url?: string;
}

/** True when the platform exposes the native share sheet (mobile + some desktops). */
export function canShare(): boolean {
  return typeof navigator !== 'undefined' && typeof navigator.share === 'function';
}

/**
 * Open the system share sheet. Returns true if the share dialog opened, false if
 * unsupported or the user cancelled — callers can ignore the result.
 */
export async function share(data: ShareData): Promise<boolean> {
  if (!canShare()) return false;
  try {
    await navigator.share(data);
    haptic();
    return true;
  } catch {
    // AbortError when the user dismisses the sheet — not an error worth surfacing.
    return false;
  }
}
