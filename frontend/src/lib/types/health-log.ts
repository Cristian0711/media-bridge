import type { HealthReport, HealthStatus } from '$lib/types/health';

export interface ScanLogSummary {
  id: number;
  checked_at: string;
  status: HealthStatus;
  full_scan: boolean;
  duration_ms: number;
  fail_count: number;
  warn_count: number;
}

export interface PaginatedScanLogsResponse {
  scans: ScanLogSummary[];
  page: number;
  page_size: number;
  total_count: number;
  total_pages: number;
}

export interface ScanLogDetail {
  id: number;
  checked_at: string;
  status: HealthStatus;
  full_scan: boolean;
  duration_ms: number;
  fail_count: number;
  warn_count: number;
  report: HealthReport;
}

export interface LatestScanResponse {
  scan: ScanLogSummary | null;
}
