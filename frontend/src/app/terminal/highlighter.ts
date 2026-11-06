import type { IDisposable, Terminal } from '@xterm/xterm';

/** A rule as the terminal applies it: text color for matches. */
export interface HighlightRule {
  pattern: string;
  ignoreCase: boolean;
  color: string;
}

/** Text colors of highlights, readable on light and dark schemes. */
export const HIGHLIGHT_COLORS: Record<string, string> = {
  red: '#ef4444',
  orange: '#f97316',
  yellow: '#eab308',
  green: '#22c55e',
  teal: '#14b8a6',
  blue: '#3b82f6',
  purple: '#a855f7',
  pink: '#ec4899',
  gray: '#9ca3af',
};

/** The default set; can be turned off in the settings. */
export const BUILTIN_HIGHLIGHTS: HighlightRule[] = [
  { pattern: String.raw`\b(?:ERROR|FATAL|CRITICAL|PANIC|FAIL(?:ED|URE)?)\b`, ignoreCase: false, color: 'red' },
  { pattern: String.raw`\b(?:WARN(?:ING)?)\b`, ignoreCase: false, color: 'yellow' },
  { pattern: String.raw`\b(?:INFO|NOTICE)\b`, ignoreCase: false, color: 'blue' },
  { pattern: String.raw`\b(?:DEBUG|TRACE)\b`, ignoreCase: false, color: 'gray' },
  { pattern: String.raw`\bHTTP/\d(?:\.\d)? 5\d\d\b|" 5\d\d \d`, ignoreCase: false, color: 'red' },
  { pattern: String.raw`\bHTTP/\d(?:\.\d)? 4\d\d\b|" 4\d\d \d`, ignoreCase: false, color: 'orange' },
  { pattern: String.raw`\bHTTP/\d(?:\.\d)? 2\d\d\b|" 2\d\d \d`, ignoreCase: false, color: 'green' },
  { pattern: String.raw`\b(?:\d{1,3}\.){3}\d{1,3}(?::\d+)?\b`, ignoreCase: false, color: 'teal' },
  { pattern: String.raw`\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`, ignoreCase: true, color: 'purple' },
];

/** Longer lines are not scanned, so a rule cannot stall the terminal. */
const MAX_LINE = 1024;

export interface Span {
  start: number;
  end: number;
  color: string;
}

/** Compiles rules for JavaScript; patterns it cannot parse are skipped. */
export function compileRules(rules: HighlightRule[]): { re: RegExp; color: string }[] {
  const out: { re: RegExp; color: string }[] = [];
  for (const r of rules) {
    try {
      out.push({ re: new RegExp(r.pattern, r.ignoreCase ? 'gi' : 'g'), color: HIGHLIGHT_COLORS[r.color] ?? HIGHLIGHT_COLORS['yellow'] });
    } catch {
      // Go (RE2) syntax JavaScript does not know: the rule still works for
      // notifications, only the coloring is skipped.
    }
  }
  return out;
}

/** Non-overlapping colored spans of a line; earlier rules win. */
export function findSpans(line: string, rules: { re: RegExp; color: string }[]): Span[] {
  if (line.length > MAX_LINE) {
    return [];
  }
  const spans: Span[] = [];
  for (const { re, color } of rules) {
    re.lastIndex = 0;
    for (let m = re.exec(line); m; m = re.exec(line)) {
      if (m[0].length === 0) {
        re.lastIndex++;
        continue;
      }
      const start = m.index;
      const end = start + m[0].length;
      if (!spans.some((s) => start < s.end && end > s.start)) {
        spans.push({ start, end, color });
      }
    }
  }
  return spans.sort((a, b) => a.start - b.start);
}

/**
 * Colors matches in completed lines of the normal buffer with decorations:
 * the bytes and the selection are untouched. Full-screen programs (the
 * alternate buffer) are left alone.
 */
export class Highlighter {
  private rules: { re: RegExp; color: string }[] = [];
  private next = 0;
  private readonly listener: IDisposable;

  constructor(private readonly term: Terminal) {
    this.listener = term.onWriteParsed(() => this.scan());
  }

  setRules(rules: HighlightRule[]): void {
    this.rules = compileRules(rules);
  }

  dispose(): void {
    this.listener.dispose();
  }

  private scan(): void {
    const buf = this.term.buffer.active;
    if (buf.type !== 'normal' || this.rules.length === 0) {
      this.next = buf.baseY + buf.cursorY;
      return;
    }
    const end = buf.baseY + buf.cursorY; // the cursor line is still being written
    if (this.next > end) {
      this.next = end; // cleared screen or trimmed scrollback
    }
    for (let y = this.next; y < end; y++) {
      const line = buf.getLine(y);
      if (!line) {
        continue;
      }
      for (const span of findSpans(line.translateToString(true), this.rules)) {
        const marker = this.term.registerMarker(y - (buf.baseY + buf.cursorY));
        if (marker) {
          this.term.registerDecoration({ marker, x: span.start, width: span.end - span.start, foregroundColor: span.color, layer: 'top' });
        }
      }
    }
    this.next = end;
  }
}
