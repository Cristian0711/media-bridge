import type { AppSseEnvelope, AppSseEventType } from '$lib/sse/types';

export type ParsedSseMessage = {
  event: AppSseEventType | string;
  data: AppSseEnvelope | Record<string, unknown> | null;
};

/** Incrementally parse SSE frames from a text buffer. */
export function parseSseChunk(buffer: string): { messages: ParsedSseMessage[]; rest: string } {
  const messages: ParsedSseMessage[] = [];
  let rest = buffer;

  while (true) {
    const sep = rest.indexOf('\n\n');
    if (sep === -1) break;

    const block = rest.slice(0, sep);
    rest = rest.slice(sep + 2);

    // SSE comment frames (": keepalive") — ignore.
    if (block.startsWith(':')) {
      continue;
    }

    let event = 'message';
    const dataLines: string[] = [];
    for (const line of block.split('\n')) {
      if (line.startsWith('event:')) {
        event = line.slice(6).trim();
      } else if (line.startsWith('data:')) {
        dataLines.push(line.slice(5).trim());
      }
    }

    if (dataLines.length === 0) continue;

    try {
      const data = JSON.parse(dataLines.join('\n')) as AppSseEnvelope;
      messages.push({ event: (data.type ?? event) as AppSseEventType, data });
    } catch {
      messages.push({ event, data: null });
    }
  }

  return { messages, rest };
}
