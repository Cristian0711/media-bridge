/** Normalize poster paths from API into absolute HTTPS URLs. */
export function normalizePosterUrl(url?: string | null): string | undefined {
  if (!url) return undefined;
  if (url.startsWith('http://') || url.startsWith('https://')) return url;
  if (url.startsWith('//')) return `https:${url}`;
  return `https://${url}`;
}

/** TMDB rendition width. The backend emits w500 posters; we downscale per use. */
export type PosterWidth = 'w92' | 'w154' | 'w185' | 'w342' | 'w500' | 'original';

/** Width for the 44px list thumbnails (library / requests / search rows). */
export const POSTER_THUMB_WIDTH: PosterWidth = 'w185';
/** Width for the ~120px Discover strip cards. */
export const POSTER_CARD_WIDTH: PosterWidth = 'w342';

/**
 * Normalize a poster URL and, when it's a TMDB image URL, swap the rendition
 * width (`…/t/p/w500/…` → `…/t/p/<width>/…`). The backend always hands us a
 * single oversized `w500`, but list thumbnails render at 44px — requesting a
 * smaller rendition cuts poster bytes by ~80% with no visible change.
 * Non-TMDB or missing URLs pass through unchanged.
 */
export function posterAtWidth(
  url: string | null | undefined,
  width: PosterWidth,
): string | undefined {
  const abs = normalizePosterUrl(url);
  if (!abs) return undefined;
  return abs.replace(/\/t\/p\/w\d+\//, `/t/p/${width}/`);
}
