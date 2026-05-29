import { writable } from 'svelte/store';

export type SseConnectionStatus = 'disconnected' | 'connecting' | 'connected';

export const sseConnectionStatus = writable<SseConnectionStatus>('disconnected');

export function setSseConnectionStatus(status: SseConnectionStatus): void {
  sseConnectionStatus.set(status);
}
