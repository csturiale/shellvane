/**
 * Fuzzy matching for the command palette: every query character must appear
 * in order. Matches at word starts and consecutive runs score higher, so
 * "wp" ranks "web-prod" above "swap".
 */
export function fuzzyScore(query: string, text: string): number | null {
  const q = query.toLowerCase().replace(/\s+/g, '');
  const t = text.toLowerCase();
  if (!q) {
    return 0;
  }
  let score = 0;
  let ti = 0;
  let prev = -2;
  for (const ch of q) {
    const found = t.indexOf(ch, ti);
    if (found < 0) {
      return null;
    }
    const wordStart = found === 0 || /[\s\-_./@:]/.test(t[found - 1]);
    score += 1 + (wordStart ? 3 : 0) + (found === prev + 1 ? 2 : 0);
    prev = found;
    ti = found + 1;
  }
  // Prefer shorter texts when scores tie.
  return score - t.length / 100;
}

/** Sorts items by score, dropping those that do not match. */
export function fuzzyFilter<T>(query: string, items: T[], text: (item: T) => string): T[] {
  return items
    .map((item) => ({ item, score: fuzzyScore(query, text(item)) }))
    .filter((x): x is { item: T; score: number } => x.score !== null)
    .sort((a, b) => b.score - a.score)
    .map((x) => x.item);
}
