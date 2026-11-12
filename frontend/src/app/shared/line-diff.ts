/** A line of a diff: kept, added in the new version, or removed from the old one. */
export interface DiffLine {
  kind: 'same' | 'added' | 'removed';
  text: string;
}

/**
 * The lines of before and after as a diff (longest common subsequence):
 * enough for the few dozen commands of a shared item.
 */
export function lineDiff(before: string[], after: string[]): DiffLine[] {
  const n = before.length;
  const m = after.length;
  const lcs: number[][] = Array.from({ length: n + 1 }, () => new Array<number>(m + 1).fill(0));
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      lcs[i][j] = before[i] === after[j] ? lcs[i + 1][j + 1] + 1 : Math.max(lcs[i + 1][j], lcs[i][j + 1]);
    }
  }
  const out: DiffLine[] = [];
  let i = 0;
  let j = 0;
  while (i < n && j < m) {
    if (before[i] === after[j]) {
      out.push({ kind: 'same', text: before[i] });
      i++;
      j++;
    } else if (lcs[i + 1][j] >= lcs[i][j + 1]) {
      out.push({ kind: 'removed', text: before[i++] });
    } else {
      out.push({ kind: 'added', text: after[j++] });
    }
  }
  while (i < n) {
    out.push({ kind: 'removed', text: before[i++] });
  }
  while (j < m) {
    out.push({ kind: 'added', text: after[j++] });
  }
  return out;
}
