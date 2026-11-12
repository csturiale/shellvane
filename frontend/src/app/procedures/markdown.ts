import { Marked, Tokens } from 'marked';

/** A block of code of a procedure. */
export interface CodeBlock {
  index: number;
  lang: string;
  /** The text as written. */
  code: string;
  /** What is sent to a terminal: prompts ($ …) and their output removed. */
  command: string;
  /** A shell command (Paste and Run), otherwise only Copy. */
  shell: boolean;
}

export interface Procedure {
  html: string;
  blocks: CodeBlock[];
}

/** Languages of commands typed in a terminal ('' = not said). */
const SHELLS = new Set(['', 'sh', 'bash', 'zsh', 'shell', 'console', 'shell-session', 'terminal', 'fish', 'ksh', 'powershell', 'pwsh', 'ps1', 'ps', 'bat', 'cmd', 'batch']);

const esc = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

/**
 * The command of a block. In a session transcript (console, or every
 * non-empty line starting with "$ ") only the prompt lines are kept, without
 * the prompt; other lines are their output.
 */
export function commandOf(code: string, lang: string): string {
  const lines = code.replace(/\r\n/g, '\n').split('\n');
  const prompted = lines.filter((l) => /^\s*(\$|PS [^>]*>|>)\s/.test(l));
  const transcript = ['console', 'shell-session', 'terminal'].includes(lang) || (prompted.length > 0 && prompted.length === lines.filter((l) => l.trim()).length);
  if (!transcript || prompted.length === 0) {
    return code.replace(/\n+$/, '');
  }
  return prompted.map((l) => l.replace(/^\s*(\$|PS [^>]*>|>)\s/, '')).join('\n');
}

/** Placeholders of a command: {{name}} (as in snippets) and <name>. */
const PLACEHOLDER = /\{\{\s*([A-Za-z_][\w.-]*)\s*(?::[^}]*)?\}\}|<([A-Za-z][\w.-]*)>/g;

export function placeholders(command: string): string[] {
  const out: string[] = [];
  for (const m of command.matchAll(PLACEHOLDER)) {
    const name = m[1] ?? m[2];
    if (!out.includes(name)) {
      out.push(name);
    }
  }
  return out;
}

/** Fills the placeholders that have a value; the others stay as written. */
export function fill(command: string, values: Record<string, string>): string {
  return command.replace(PLACEHOLDER, (all, a: string | undefined, b: string | undefined) => {
    const name = (a ?? b)!;
    return values[name] !== undefined && values[name] !== '' ? values[name] : all;
  });
}

/** Resolves a link of a procedure to another one (relative .md path), or null. */
export function docLink(href: string, from: string): string | null {
  if (/^[a-z][\w+.-]*:/i.test(href) || href.startsWith('#') || href.startsWith('/')) {
    return null;
  }
  const path = href.split('#')[0];
  if (!/\.(md|markdown|mdx)$/i.test(path)) {
    return null;
  }
  const parts = from.split('/').slice(0, -1);
  for (const p of path.split('/')) {
    if (p === '..') {
      parts.pop();
    } else if (p && p !== '.') {
      parts.push(decodeURIComponent(p));
    }
  }
  return parts.join('/');
}

/**
 * Renders a procedure. Its text is not trusted: raw HTML is shown as text,
 * images are left out (their alt text stays) and only web and relative
 * links are kept, so the output can be inserted as it is. Code blocks get
 * their actions, handled by the panel through data attributes.
 */
export function renderProcedure(markdown: string, path: string, labels: { paste: string; run: string; copy: string; done: string }): Procedure {
  const blocks: CodeBlock[] = [];
  const md = new Marked({ gfm: true });
  md.use({
    renderer: {
      html(token: Tokens.HTML | Tokens.Tag): string {
        return esc(token.text);
      },
      image(token: Tokens.Image): string {
        return esc(token.text);
      },
      link(token: Tokens.Link): string {
        const text = this.parser.parseInline(token.tokens);
        const doc = docLink(token.href, path);
        if (doc !== null) {
          return `<a href="#" class="doc-link" data-doc="${esc(doc)}">${text}</a>`;
        }
        if (/^(https?:|mailto:)/i.test(token.href)) {
          return `<a href="${esc(token.href)}" rel="noopener">${text}</a>`;
        }
        return text;
      },
      code(token: Tokens.Code): string {
        const lang = (token.lang ?? '').trim().split(/\s+/)[0].toLowerCase();
        const shell = SHELLS.has(lang);
        const block: CodeBlock = { index: blocks.length, lang, code: token.text, command: shell ? commandOf(token.text, lang) : token.text, shell };
        blocks.push(block);
        const i = block.index;
        const actions = shell
          ? `<button type="button" class="btn small" data-act="paste" data-i="${i}">${esc(labels.paste)}</button><button type="button" class="btn small primary" data-act="run" data-i="${i}">${esc(labels.run)}</button>`
          : `<button type="button" class="btn small" data-act="copy" data-i="${i}">${esc(labels.copy)}</button>`;
        return `<div class="proc-code" data-i="${i}"><div class="proc-bar"><span class="proc-n">${i + 1}</span><span class="proc-lang">${esc(lang)}</span><button type="button" class="proc-done" data-act="done" data-i="${i}" title="${esc(labels.done)}" aria-label="${esc(labels.done)}"></button>${actions}</div><pre><code>${esc(token.text)}</code></pre></div>\n`;
      },
    },
  });
  const html = md.parse(markdown, { async: false }) as string;
  return { html, blocks };
}
