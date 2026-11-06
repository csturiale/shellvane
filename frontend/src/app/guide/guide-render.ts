import { Marked, Tokens } from 'marked';

import { ANCHOR } from './guide';

/**
 * Renders a topic. Headings get their anchor as id; links to "shellvane:…"
 * become in-app actions (handled by the view), other links open outside.
 */
export function render(markdown: string): string {
  const md = new Marked({ gfm: true });
  md.use({
    renderer: {
      heading(token: Tokens.Heading): string {
        const text = this.parser.parseInline(token.tokens);
        const m = ANCHOR.exec(token.text);
        const label = text.replace(/\s*\{#[\w-]+\}\s*$/, '');
        return `<h${token.depth}${m ? ` id="${m[1]}"` : ''}>${label}</h${token.depth}>\n`;
      },
      link(token: Tokens.Link): string {
        const text = this.parser.parseInline(token.tokens);
        if (token.href.startsWith('shellvane:')) {
          return `<a href="#" class="action" data-action="${escapeAttr(token.href.slice('shellvane:'.length))}">${text}</a>`;
        }
        return `<a href="${escapeAttr(token.href)}" target="_blank" rel="noopener">${text}</a>`;
      },
    },
  });
  return md.parse(markdown, { async: false }) as string;
}

function escapeAttr(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;');
}
