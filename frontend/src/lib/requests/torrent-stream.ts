import { getToken } from '$lib/auth/session';
import { parseSseChunk } from '$lib/sse/parse';
import type { RequestTorrentInfo } from '$lib/types/torrent';

export type TorrentStreamMessage = {
  type: 'connected' | 'torrent.update' | 'error';
  request_id?: number;
  payload?: RequestTorrentInfo;
  error?: string;
};

export type RequestTorrentStream = {
  close: () => void;
};

/**
 * Opens GET /api/v1/requests/:id/torrent/events — server pushes fresh status every second.
 */
export function connectRequestTorrentStream(
  requestId: number,
  handlers: {
    onUpdate: (info: RequestTorrentInfo) => void;
    onError?: (message: string) => void;
    onConnected?: () => void;
  },
): RequestTorrentStream {
  if (typeof window === 'undefined') {
    return { close: () => {} };
  }

  let closed = false;
  let abort: AbortController | null = null;

  const run = async () => {
    if (closed) return;

    const token = getToken();
    if (!token) {
      handlers.onError?.('Not authenticated');
      return;
    }

    abort = new AbortController();

    try {
      const res = await fetch(`/api/v1/requests/${requestId}/torrent/events`, {
        headers: { Authorization: `Bearer ${token}`, Accept: 'text/event-stream' },
        signal: abort.signal,
      });

      if (!res.ok || !res.body) {
        handlers.onError?.(res.statusText || 'Stream failed');
        return;
      }

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
          const raw = msg.data as TorrentStreamMessage | null;
          if (!raw || typeof raw.type !== 'string') continue;

          switch (raw.type) {
            case 'connected':
              handlers.onConnected?.();
              break;
            case 'torrent.update':
              if (raw.payload) {
                handlers.onUpdate(raw.payload);
              }
              break;
            case 'error':
              handlers.onError?.(raw.error ?? 'Stream error');
              break;
          }
        }
      }
    } catch (err) {
      if (closed || (err instanceof DOMException && err.name === 'AbortError')) {
        return;
      }
      handlers.onError?.(err instanceof Error ? err.message : 'Stream disconnected');
    }
  };

  void run();

  return {
    close: () => {
      closed = true;
      abort?.abort();
    },
  };
}
