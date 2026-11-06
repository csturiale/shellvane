import { BUILTIN_HIGHLIGHTS, HIGHLIGHT_COLORS, compileRules, findSpans } from './highlighter';

describe('highlighter', () => {
  const builtin = compileRules(BUILTIN_HIGHLIGHTS);

  it('finds log levels, addresses, HTTP codes and UUIDs', () => {
    const line = '2026-10-31 ERROR 10.0.0.1:443 "GET / HTTP/1.1" 503 12 id=123e4567-e89b-12d3-a456-426614174000';
    const found = findSpans(line, builtin).map((s) => line.slice(s.start, s.end));
    expect(found).toEqual(['ERROR', '10.0.0.1:443', '" 503 1', '123e4567-e89b-12d3-a456-426614174000']);
  });

  it('keeps the first rule on overlaps and skips invalid patterns', () => {
    const rules = compileRules([
      { pattern: 'abc', ignoreCase: false, color: 'red' },
      { pattern: 'bcd', ignoreCase: false, color: 'blue' },
      { pattern: '(?<!x', ignoreCase: false, color: 'green' },
      { pattern: 'XYZ', ignoreCase: true, color: 'nope' },
    ]);
    expect(rules).toHaveLength(3);
    expect(findSpans('abcd xyz', rules)).toEqual([
      { start: 0, end: 3, color: HIGHLIGHT_COLORS['red'] },
      { start: 5, end: 8, color: HIGHLIGHT_COLORS['yellow'] },
    ]);
  });

  it('ignores empty matches and very long lines', () => {
    expect(findSpans('aaa', compileRules([{ pattern: 'x*', ignoreCase: false, color: 'red' }]))).toEqual([]);
    expect(findSpans('ERROR'.repeat(500), builtin)).toEqual([]);
  });
});
