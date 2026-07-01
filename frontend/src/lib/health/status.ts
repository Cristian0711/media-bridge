import type { CheckStatus, HealthStatus } from '$lib/types/health';

export function overallStatusLabel(status: HealthStatus): string {
  switch (status) {
    case 'healthy':
      return 'All systems operational';
    case 'degraded':
      return 'Degraded';
    case 'unhealthy':
      return 'Unhealthy';
    default:
      return status;
  }
}

export function overallStatusClass(status: HealthStatus): string {
  switch (status) {
    case 'healthy':
      return 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30';
    case 'degraded':
      return 'bg-amber-500/10 text-amber-400 border-amber-500/30';
    case 'unhealthy':
      return 'bg-red-500/10 text-red-400 border-red-500/30';
    default:
      return 'bg-white/10 text-white/70 border-white/15';
  }
}

export function checkStatusClass(status: CheckStatus): string {
  switch (status) {
    case 'ok':
      return 'text-emerald-400';
    case 'warn':
      return 'text-amber-400';
    case 'fail':
      return 'text-red-400';
    case 'skip':
      return 'text-white/45';
    default:
      return 'text-white/70';
  }
}

export function checkStatusIcon(status: CheckStatus): 'ok' | 'warn' | 'fail' | 'skip' | 'unknown' {
  switch (status) {
    case 'ok':
      return 'ok';
    case 'warn':
      return 'warn';
    case 'fail':
      return 'fail';
    case 'skip':
      return 'skip';
    default:
      return 'unknown';
  }
}
