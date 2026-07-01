import { writable } from 'svelte/store';

export const MEDIA_UNAVAILABLE = "This media isn't available yet!";

export const NO_FREELEECH =
  'No freeleech torrent available for this quality.';

type ToastItem = {
  id: number;
  message: string;
};

export const toasts = writable<ToastItem[]>([]);

export function showToast(message: string, durationMs = 3500): void {
  const id = Date.now() + Math.floor(Math.random() * 1000);
  toasts.update((items) => [...items, { id, message }]);
  setTimeout(() => {
    toasts.update((items) => items.filter((t) => t.id !== id));
  }, durationMs);
}
