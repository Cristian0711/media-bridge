export type HealthStatus = 'healthy' | 'degraded' | 'unhealthy';
export type CheckStatus = 'ok' | 'warn' | 'fail' | 'skip';

export interface HealthCheck {
  id: string;
  name: string;
  status: CheckStatus;
  message: string;
  duration_ms: number;
  details?: Record<string, unknown>;
}

export interface HealthReport {
  status: HealthStatus;
  checked_at: string;
  checks: HealthCheck[];
}

export interface LinkIssueSample {
  path: string;
  zone: string;
  nlink: number;
  reason: string;
}
