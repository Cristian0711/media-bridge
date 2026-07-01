import type { ApiMedia } from '$lib/types/media-library';
import type { RequestRow } from '$lib/types/request';

export type AppSseEventType =
  | 'connected'
  | 'heartbeat'
  | 'media.added'
  | 'media.removed'
  | 'request.created'
  | 'request.status_changed';

export type AppSseEnvelope = {
  type: AppSseEventType;
  timestamp?: number;
  media?: ApiMedia;
  request?: RequestRow;
};
