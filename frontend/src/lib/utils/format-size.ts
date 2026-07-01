/** Formats a byte count with adaptive units (B, KB, MB, GB, TB). */
export function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let v = bytes;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

/** Formats a byte count in binary GB with precision that scales down as the
 *  number grows ("2.34 GB", "12.3 GB", "120 GB"); null for missing/zero sizes. */
export function formatSizeGB(bytes?: number): string | null {
  if (!bytes || bytes <= 0) return null;
  const gb = bytes / 1024 ** 3;
  if (gb >= 100) return `${Math.round(gb)} GB`;
  if (gb >= 10) return `${gb.toFixed(1)} GB`;
  return `${gb.toFixed(2)} GB`;
}

/** Formats a byte count as fixed 2-decimal binary GB ("2.34 GB").
 *  Used for torrent result rows where a consistent width is wanted. */
export function formatGbFixed(bytes: number): string {
  return `${(bytes / 1024 ** 3).toFixed(2)} GB`;
}
