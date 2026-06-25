/** List kind after the colon (e.g. netflix:drama-series → drama-series). */
export function listKind(id: string): string {
  const i = id.indexOf(':');
  return i >= 0 ? id.slice(i + 1) : id;
}

/** Whether a browse list holds series rather than movies, by its id kind. */
export function isSeriesList(id: string): boolean {
  const kind = listKind(id);
  return kind === 'series' || kind.endsWith('-series') || kind === 'trending-series';
}
