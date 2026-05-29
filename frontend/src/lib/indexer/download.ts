import type { IndexerMovie, IndexerShow } from '$lib/types/indexer';

const QUALITY_ORDER = [
  '4K Remux',
  '4K BluRay',
  '4K',
  '1080p Remux',
  '1080p BluRay',
  '1080p',
  '720p Remux',
  '720p',
  'Unknown',
];

export function sortQualities(qualities: string[]): string[] {
  const order = new Map(QUALITY_ORDER.map((q, i) => [q, i]));
  return [...qualities].sort((a, b) => (order.get(a) ?? 99) - (order.get(b) ?? 99));
}

export function freeleechMovieQualities(movies: IndexerMovie[]): string[] {
  const set = new Set<string>();
  for (const m of movies) {
    if (m.seeders > 0 && m.freeleech === 1) {
      set.add(m.quality);
    }
  }
  return sortQualities([...set]);
}

export function freeleechShowQualities(shows: IndexerShow[]): string[] {
  const set = new Set<string>();
  for (const s of shows) {
    if (s.seeders > 0 && s.freeleech === 1) {
      set.add(s.quality);
    }
  }
  return sortQualities([...set]);
}

export function hasSeeders(movies: IndexerMovie[]): boolean {
  return movies.some((m) => m.seeders > 0);
}

export function hasSeedersShow(shows: IndexerShow[]): boolean {
  return shows.some((s) => s.seeders > 0);
}
