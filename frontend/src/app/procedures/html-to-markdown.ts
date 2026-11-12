/**
 * Turns the content of a web page (Confluence, a wiki, documentation) into
 * Markdown: headings, paragraphs, lists, tables, links and, above all, code
 * blocks with their language.
 *
 * It is self-contained (no imports, no helpers outside it) because its source
 * text is also what the "Send to Shellvane" bookmarklet runs in the browser.
 */
export function htmlToMarkdown(root: Node): string {
  const SKIP = /^(SCRIPT|STYLE|NOSCRIPT|SVG|BUTTON|NAV|FOOTER|FORM|IFRAME|SELECT|TEXTAREA|INPUT|IMG|VIDEO|AUDIO|CANVAS|TEMPLATE)$/;
  const BLOCK = /^(P|DIV|SECTION|ARTICLE|MAIN|HEADER|ASIDE|BLOCKQUOTE|UL|OL|LI|TABLE|THEAD|TBODY|TR|PRE|H[1-6]|HR|DL|DT|DD|FIGURE|DETAILS|SUMMARY)$/;
  const langOf = (el: Element): string => {
    let e: Element | null = el;
    for (let i = 0; e && i < 4; i++, e = e.parentElement) {
      const attr = e.getAttribute('data-language') || e.getAttribute('data-code-lang') || e.getAttribute('data-lang') || '';
      if (attr) {
        return attr.toLowerCase();
      }
      const params = e.getAttribute('data-syntaxhighlighter-params') || '';
      const brush = /brush:\s*([\w+-]+)/.exec(params + ' ' + (e.getAttribute('class') || ''));
      if (brush) {
        return brush[1].toLowerCase();
      }
      const cls = /(?:^|\s)(?:language|lang|highlight-source)-([\w+-]+)/.exec(e.getAttribute('class') || '');
      if (cls) {
        return cls[1].toLowerCase();
      }
    }
    return '';
  };
  const inline = (node: Node): string => {
    if (node.nodeType === 3) {
      return (node.nodeValue || '').replace(/\s+/g, ' ');
    }
    if (node.nodeType !== 1) {
      return '';
    }
    const el = node as Element;
    const tag = el.tagName.toUpperCase();
    if (SKIP.test(tag)) {
      return '';
    }
    if (tag === 'BR') {
      return '\n';
    }
    let text = '';
    el.childNodes.forEach((c) => (text += inline(c)));
    if (tag === 'CODE' || tag === 'KBD' || tag === 'TT') {
      const t = (el.textContent || '').replace(/\s+/g, ' ').trim();
      return t ? '`' + t.replace(/`/g, 'ˋ') + '`' : '';
    }
    if ((tag === 'STRONG' || tag === 'B') && text.trim()) {
      return '**' + text.trim() + '** ';
    }
    if ((tag === 'EM' || tag === 'I') && text.trim()) {
      return '_' + text.trim() + '_ ';
    }
    if (tag === 'A') {
      const href = (el as HTMLAnchorElement).href || '';
      return /^https?:/i.test(href) && text.trim() ? '[' + text.trim() + '](' + href + ')' : text;
    }
    return text;
  };
  const isCode = (el: Element): boolean => {
    const tag = el.tagName.toUpperCase();
    if (tag === 'PRE') {
      return true;
    }
    const cls = el.getAttribute('class') || '';
    return /(^|\s)(code-block|codeContent|code-content|syntaxhighlighter|highlight)(\s|$)/i.test(cls) && !el.querySelector('pre');
  };
  const lines: string[] = [];
  const out = (s: string, indent: string) => {
    for (const l of s.split('\n')) {
      lines.push(l ? indent + l : '');
    }
  };
  const block = (node: Node, indent: string): void => {
    if (node.nodeType !== 1) {
      const t = inline(node).trim();
      if (t) {
        out(t, indent);
        lines.push('');
      }
      return;
    }
    const el = node as Element;
    const tag = el.tagName.toUpperCase();
    if (SKIP.test(tag) || el.getAttribute('aria-hidden') === 'true') {
      return;
    }
    if (isCode(el)) {
      const code = ((el as HTMLElement).innerText || el.textContent || '').replace(/ /g, ' ').replace(/\n+$/, '');
      if (code.trim()) {
        const fence = code.includes('```') ? '~~~' : '```';
        out(fence + langOf(el.querySelector('code') || el) + '\n' + code + '\n' + fence, indent);
        lines.push('');
      }
      return;
    }
    const h = /^H([1-6])$/.exec(tag);
    if (h) {
      const t = inline(el).trim();
      if (t) {
        out('#'.repeat(+h[1]) + ' ' + t, indent);
        lines.push('');
      }
      return;
    }
    if (tag === 'UL' || tag === 'OL') {
      let n = 1;
      for (const li of Array.from(el.children)) {
        if (li.tagName.toUpperCase() !== 'LI') {
          continue;
        }
        const mark = tag === 'OL' ? n++ + '. ' : '- ';
        const pad = ' '.repeat(mark.length);
        const start = lines.length;
        flow(li, indent + pad);
        while (lines.length > start && lines[lines.length - 1] === '') {
          lines.pop();
        }
        if (lines.length === start) {
          continue;
        }
        lines[start] = indent + mark + lines[start].slice((indent + pad).length);
      }
      lines.push('');
      return;
    }
    if (tag === 'TABLE') {
      const rows = Array.from(el.querySelectorAll('tr')).map((tr) =>
        Array.from(tr.children).map((c) => inline(c).trim().replace(/\|/g, '\\|').replace(/\n/g, ' ')),
      );
      if (rows.length) {
        out('| ' + rows[0].join(' | ') + ' |', indent);
        out('|' + rows[0].map(() => ' --- |').join(''), indent);
        rows.slice(1).forEach((r) => out('| ' + r.join(' | ') + ' |', indent));
        lines.push('');
      }
      return;
    }
    if (tag === 'HR') {
      out('---', indent);
      lines.push('');
      return;
    }
    if (tag === 'BLOCKQUOTE') {
      const start = lines.length;
      flow(el, indent);
      for (let i = start; i < lines.length; i++) {
        lines[i] = lines[i] ? indent + '> ' + lines[i].slice(indent.length) : '';
      }
      return;
    }
    flow(el, indent);
  };
  // Children of a block: runs of inline content become paragraphs.
  const flow = (el: Node, indent: string): void => {
    let run = '';
    const flush = () => {
      const t = run.replace(/[ \t]+\n/g, '\n').replace(/\n{3,}/g, '\n\n').trim();
      if (t) {
        out(t, indent);
        lines.push('');
      }
      run = '';
    };
    el.childNodes.forEach((c) => {
      if (c.nodeType === 1 && (BLOCK.test((c as Element).tagName.toUpperCase()) || isCode(c as Element))) {
        flush();
        block(c, indent);
      } else {
        run += inline(c);
      }
    });
    flush();
  };
  flow(root, '');
  return lines
    .join('\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim();
}

/** The main content of a page: the selection, else the article of known wikis, else the body. */
export function pageContent(doc: Document): Node {
  const sel = doc.getSelection();
  if (sel && sel.rangeCount > 0 && !sel.isCollapsed) {
    const div = doc.createElement('div');
    div.appendChild(sel.getRangeAt(0).cloneContents());
    return div;
  }
  return doc.querySelector('#main-content, .wiki-content, [data-testid="renderer-page"], .ak-renderer-document, article, main, [role="main"]') || doc.body;
}
