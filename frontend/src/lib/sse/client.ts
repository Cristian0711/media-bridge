import { getToken } from '$lib/auth/session';
import { setSseConnectionStatus } from '$lib/sse/connection-status';
import { dispatchAppSseEvent } from '$lib/sse/dispatch';
import { parseSseChunk } from '$lib/sse/parse';
import { syncListsAfterSseReconnect } from '$lib/sse/reconnect-sync';
import type { AppSseEnvelope } from '$lib/sse/types';

const RECONNECT_MS_MIN = 2000;
const RECONNECT_MS_MAX = 30000;

export type AppSseConnection = {
  close: () => void;
};

/**
 * Opens GET /api/v1/events with Bearer auth (fetch — EventSource cannot set headers).
 * Stays on one long-lived connection; reconnects only if the stream drops unexpectedly.
 */
export function connectAppEvents(): AppSseConnection {
  if (typeof window === 'undefined') {
    return { close: () => {} };
  }

  let closed = false;
  let abort: AbortController | null = null;
  let reconnectDelay = RECONNECT_MS_MIN;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  /** True after the first successful stream in this session (skip sync on cold connect). */
  let hadSuccessfulConnection = false;

  const scheduleReconnect = () => {
    if (closed) return;
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null;
      void run();
    }, reconnectDelay);
    reconnectDelay = Math.min(reconnectDelay * 1.5, RECONNECT_MS_MAX);
  };

  const run = async () => {
    if (closed) return;

    const token = getToken();
    if (!token) {
      setSseConnectionStatus('disconnected');
      return;
    }

    setSseConnectionStatus('connecting');
    abort = new AbortController();

    try {
      const res = await fetch('/api/v1/events', {
        headers: { Authorization: `Bearer ${token}`, Accept: 'text/event-stream' },
        credentials: 'include',
        signal: abort.signal,
      });

      if (!res.ok || !res.body) {
        setSseConnectionStatus('connecting');
        scheduleReconnect();
        return;
      }

      reconnectDelay = RECONNECT_MS_MIN;
      setSseConnectionStatus('connected');
      if (hadSuccessfulConnection) {
        syncListsAfterSseReconnect();
      }
      hadSuccessfulConnection = true;
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';

      while (!closed) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const { messages, rest } = parseSseChunk(buffer);
        buffer = rest;

        for (const msg of messages) {
          dispatchAppSseEvent(msg.data as AppSseEnvelope | null);
        }
      }

      // Stream ended without an explicit client close — reconnect.
      if (!closed && !abort.signal.aborted) {
        setSseConnectionStatus('connecting');
        scheduleReconnect();
      }
    } catch (err) {
      if (closed || (err instanceof DOMException && err.name === 'AbortError')) {
        return;
      }
      setSseConnectionStatus('connecting');
      scheduleReconnect();
    }
  };

  void run();

  return {
    close: () => {
      closed = true;
      hadSuccessfulConnection = false;
      setSseConnectionStatus('disconnected');
      if (reconnectTimer) clearTimeout(reconnectTimer);
      abort?.abort();
    },
  };
}
