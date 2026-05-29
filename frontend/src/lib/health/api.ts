import { callApi } from '$lib/api/client';
import type { HealthReport } from '$lib/types/health';
import type {
  LatestScanResponse,
  PaginatedScanLogsResponse,
  ScanLogDetail,
} from '$lib/types/health-log';

export function getHealthReport(full = false, persist = false): Promise<HealthReport> {
  const params = new URLSearchParams();
  if (full) params.set('full', '1');
  if (persist) params.set('persist', '1');
  const q = params.toString();
  return callApi<HealthReport>(`/health${q ? `?${q}` : ''}`);
}

export function listHealthScans(page = 1, pageSize = 30): Promise<PaginatedScanLogsResponse> {
  return callApi<PaginatedScanLogsResponse>(
    `/health/scans?page=${page}&page_size=${pageSize}`,
  );
}

export function getLatestHealthScan(): Promise<LatestScanResponse> {
  return callApi<LatestScanResponse>('/health/scans/latest');
}

export function getHealthScan(id: number): Promise<ScanLogDetail> {
  return callApi<ScanLogDetail>(`/health/scans/${id}`);
}
