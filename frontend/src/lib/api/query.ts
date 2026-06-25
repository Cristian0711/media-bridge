/** Builds a query string ("?a=1&b=2") from a params object, skipping entries
 *  that are undefined, null, or empty string. Returns "" when nothing is set.
 *  Pass already-encoded API key names (e.g. `page_size`); callers should map
 *  falsy-but-meaningless values to `undefined` to omit them. */
export function buildQuery(
  params: Record<string, string | number | undefined | null>,
): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === '') continue;
    search.set(key, String(value));
  }
  const qs = search.toString();
  return qs ? `?${qs}` : '';
}
