// Tactile feedback for touch interactions. `navigator.vibrate` is Android-only
// (iOS Safari doesn't implement it), so this is feature-detected and no-ops
// everywhere else — callers don't need to guard.

/** A short tap for discrete actions (tab switch, gesture commit, button press). */
export const HAPTIC_TAP = 10;

/** Feed a vibration pattern to the device if supported; silently no-op if not. */
export function haptic(pattern: number | number[] = HAPTIC_TAP): void {
  if (typeof navigator === 'undefined' || !('vibrate' in navigator)) return;
  navigator.vibrate(pattern);
}
