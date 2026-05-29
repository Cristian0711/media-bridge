import type { RequestRow } from '$lib/types/request';

export function posterUrl(url?: string): string | undefined {
  if (!url) return undefined;
  if (url.startsWith('http://') || url.startsWith('https://')) return url;
  if (url.startsWith('//')) return `https:${url}`;
  return `https://${url}`;
}

export function isShowRequest(type: string): boolean {
  return type.startsWith('show_');
}

export function requestActionLabel(type: string): string {
  if (type.includes('remove')) return 'Remove';
  if (type.includes('download')) return 'Download';
  return type.replaceAll('_', ' ');
}

export function requestMediaKind(type: string): string {
  if (type.startsWith('movie_')) return 'Movie';
  if (type.startsWith('show_')) return 'Show';
  return '';
}

/** @deprecated Use requestActionLabel + requestMediaKind for compact UI */
export function requestTypeLabel(type: string): string {
  const action = requestActionLabel(type);
  const kind = requestMediaKind(type);
  if (kind) return `${action} ${kind.toLowerCase()}`;
  return action;
}

export function showScope(req: RequestRow): string {
  if (!isShowRequest(req.type)) return '';
  if (req.season && req.episode) {
    return `S${String(req.season).padStart(2, '0')}E${String(req.episode).padStart(2, '0')}`;
  }
  if (req.season) return `Season ${req.season}`;
  return 'Full series';
}

export function formatRelativeTime(dateString: string): string {
  try {
    const date = new Date(dateString);
    if (Number.isNaN(date.getTime())) return 'Unknown';

    const diffMs = Date.now() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return 'Just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    if (diffDays < 7) return `${diffDays}d ago`;

    return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
  } catch {
    return 'Unknown';
  }
}

export function statusBadgeClass(status: string): string {
  switch (status) {
    case 'pending':
      return 'bg-yellow-500/10 text-yellow-400 border-yellow-500/25';
    case 'queued':
      return 'bg-amber-500/10 text-amber-400 border-amber-500/25';
    case 'downloading':
      return 'bg-green-500/10 text-green-400 border-green-500/25';
    case 'downloaded':
      return 'bg-emerald-500/10 text-emerald-400 border-emerald-500/25';
    case 'removing':
      return 'bg-orange-500/10 text-orange-400 border-orange-500/25';
    case 'removed':
    case 'processed':
      return 'bg-blue-500/10 text-blue-400 border-blue-500/25';
    case 'cancelled':
      return 'bg-slate-500/10 text-slate-400 border-slate-500/25';
    case 'failed':
      return 'bg-red-500/10 text-red-400 border-red-500/25';
    default:
      return 'bg-white/10 text-white/70 border-white/15';
  }
}
