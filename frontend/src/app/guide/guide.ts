import { EXTENSIONS } from '../extensions/extensions';

/** Topics in reading order; each is guide/<lang>/<id>.md. */
export const GUIDE_TOPICS = [
  'getting-started',
  'hosts',
  'connecting',
  'auth',
  'jump-proxy',
  'vault',
  'keys',
  'terminal',
  'broadcast',
  'snippets',
  'tunnels',
  'files',
  'continuity',
  'automation',
  'kubernetes',
  'environments',
  'plugins',
  'runbooks',
  'integration',
  'sync',
  'import',
  'shortcuts',
  'security',
  'contribute',
  'whats-new',
] as const;
export type GuideTopic = (typeof GUIDE_TOPICS)[number];

/** Every topic: the built-in ones, then those of extensions. */
export const ALL_GUIDE_TOPICS: readonly string[] = [...GUIDE_TOPICS, ...EXTENSIONS.flatMap((e) => e.guideTopics ?? [])];

/** A heading of a topic, for the table of contents and the search. */
export interface GuideSection {
  topic: string;
  anchor: string;
  title: string;
  /** Plain text of the section, for the search. */
  text: string;
}

export const ANCHOR = /\s*\{#([\w-]+)\}\s*$/;

/**
 * Splits a topic into its sections. Headings end with {#anchor}, the same
 * in every language, so links from the app work whatever the language.
 */
export function sections(topic: string, markdown: string): GuideSection[] {
  const out: GuideSection[] = [];
  let current: GuideSection | null = null;
  for (const line of markdown.split('\n')) {
    const h = /^#{1,3}\s+(.*)$/.exec(line);
    if (h) {
      const m = ANCHOR.exec(h[1]);
      current = { topic, anchor: m?.[1] ?? '', title: h[1].replace(ANCHOR, ''), text: '' };
      out.push(current);
    } else if (current) {
      current.text += plain(line) + ' ';
    }
  }
  return out;
}

/** Markdown inline syntax removed, for searching. */
function plain(line: string): string {
  return line
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/[`*_|>#-]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim();
}

/** Case- and accent-insensitive search; titles rank before body text. */
export function search(all: GuideSection[], query: string, limit = 20): GuideSection[] {
  const words = fold(query).split(/\s+/).filter(Boolean);
  if (words.length === 0) {
    return [];
  }
  const scored: { s: GuideSection; score: number }[] = [];
  for (const s of all) {
    const title = fold(s.title);
    const text = fold(s.text);
    if (words.every((w) => title.includes(w) || text.includes(w))) {
      scored.push({ s, score: words.filter((w) => title.includes(w)).length * 10 + (s.anchor === 'top' ? 1 : 0) });
    }
  }
  return scored
    .sort((a, b) => b.score - a.score)
    .slice(0, limit)
    .map((x) => x.s);
}

function fold(s: string): string {
  return s
    .toLowerCase()
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '');
}

/** A sentence around the first match, for search results. */
export function excerpt(s: GuideSection, query: string, size = 120): string {
  const word = fold(query).split(/\s+/).find(Boolean) ?? '';
  const i = Math.max(0, fold(s.text).indexOf(word));
  const start = Math.max(0, i - size / 3);
  return (start > 0 ? '…' : '') + s.text.slice(start, start + size).trim() + (start + size < s.text.length ? '…' : '');
}

/** "view/hosts", "action/newHost", "topic/auth#totp". */
export function parseAction(action: string): { kind: string; target: string; anchor: string } {
  const [path, anchor = ''] = action.split('#');
  const [kind, target = ''] = path.split('/');
  return { kind, target, anchor };
}
