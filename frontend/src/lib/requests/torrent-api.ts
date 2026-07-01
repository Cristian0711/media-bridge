import { callApi } from '$lib/api/client';
import type { RequestTorrentInfo } from '$lib/types/torrent';

export async function getRequestTorrentInfo(requestId: number): Promise<RequestTorrentInfo> {
  return callApi<RequestTorrentInfo>(`/requests/${requestId}/torrent`);
}
