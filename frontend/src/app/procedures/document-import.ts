/**
 * Turns a document chosen on this computer into a procedure's Markdown:
 * Word (.docx) by its paragraph styles, PDF by its text and fonts, and
 * Markdown, HTML or text as they are. Commands are found as in a pasted
 * page: code styles and monospace fonts become code blocks.
 */
import { htmlToMarkdown } from './html-to-markdown';

/** A document read: its title (the file name without extension) and Markdown. */
export interface ImportedDocument {
  title: string;
  markdown: string;
}

const MONO = /mono|courier|consolas|menlo|monaco|source code|fira code|jetbrains|inconsolata|lucida console|andale/i;

export async function importDocument(file: File): Promise<ImportedDocument> {
  const title = file.name.replace(/\.[^.]+$/, '');
  const ext = (/\.([^.]+)$/.exec(file.name)?.[1] ?? '').toLowerCase();
  const data = await file.arrayBuffer();
  let markdown: string;
  switch (ext) {
    case 'docx':
      markdown = await docxToMarkdown(data);
      break;
    case 'pdf':
      markdown = await pdfToMarkdown(data);
      break;
    case 'html':
    case 'htm':
      markdown = htmlToMarkdown(new DOMParser().parseFromString(new TextDecoder().decode(data), 'text/html').body);
      break;
    default:
      markdown = new TextDecoder().decode(data); // .md, .txt
  }
  return { title, markdown: markdown.replace(/\n{3,}/g, '\n\n').trim() + '\n' };
}

// ---- Word ----------------------------------------------------------------

/** word/document.xml of a .docx as Markdown. */
export async function docxToMarkdown(data: ArrayBuffer): Promise<string> {
  const xml = await unzipEntry(data, 'word/document.xml');
  if (xml === null) {
    throw new Error('not a Word document (no word/document.xml)');
  }
  return documentXmlToMarkdown(xml);
}

const W = 'http://schemas.openxmlformats.org/wordprocessingml/2006/main';

/** The body of a WordprocessingML document as Markdown. */
export function documentXmlToMarkdown(xml: string): string {
  const doc = new DOMParser().parseFromString(xml, 'application/xml');
  const body = doc.getElementsByTagNameNS(W, 'body')[0];
  if (!body) {
    return '';
  }
  const out: string[] = [];
  let code: string[] | null = null;
  const flushCode = () => {
    if (code) {
      out.push('```\n' + code.join('\n') + '\n```');
      code = null;
    }
  };
  for (const node of Array.from(body.children)) {
    if (node.localName === 'tbl') {
      flushCode();
      out.push(tableToMarkdown(node));
      continue;
    }
    if (node.localName !== 'p') {
      continue;
    }
    const style = attr(child(child(node, 'pPr'), 'pStyle'), 'val');
    const text = paragraphText(node);
    if (isCodeParagraph(node, style)) {
      (code ??= []).push(text);
      continue;
    }
    flushCode();
    if (!text.trim()) {
      continue;
    }
    // Style ids by Word's language: Heading1, Titolo1, Überschrift1, Titre1…
    const heading = /^(?:heading|titolo|überschrift|titre|título|kop)\s*(\d)$/i.exec(style);
    if (heading) {
      out.push('#'.repeat(Math.min(+heading[1], 6)) + ' ' + text.trim());
    } else if (/^title$/i.test(style)) {
      out.push('# ' + text.trim());
    } else if (child(child(node, 'pPr'), 'numPr') || /list/i.test(style)) {
      out.push('- ' + text.trim());
    } else {
      out.push(text.trim());
    }
  }
  flushCode();
  // Consecutive list items stay together.
  return out.join('\n\n').replace(/^(- .*)\n\n(?=- )/gm, '$1\n');
}

/** A paragraph's text: runs, tabs and line breaks. */
function paragraphText(p: Element): string {
  let s = '';
  const walk = (el: Element) => {
    for (const c of Array.from(el.children)) {
      switch (c.localName) {
        case 't':
          s += c.textContent ?? '';
          break;
        case 'tab':
          s += '\t';
          break;
        case 'br':
        case 'cr':
          s += '\n';
          break;
        case 'pPr':
        case 'rPr':
          break;
        default:
          walk(c);
      }
    }
  };
  walk(p);
  return s;
}

/** A code style (Code, HTML Preformatted, Source…) or every run in a monospace font. */
function isCodeParagraph(p: Element, style: string): boolean {
  if (/code|preformatted|source|console|terminal|command|monospace/i.test(style)) {
    return true;
  }
  const runs = Array.from(p.getElementsByTagNameNS(W, 'r')).filter((r) => (r.textContent ?? '').trim());
  return runs.length > 0 && runs.every((r) => MONO.test(attr(child(child(r, 'rPr'), 'rFonts'), 'ascii')));
}

function tableToMarkdown(tbl: Element): string {
  const rows = Array.from(tbl.children)
    .filter((r) => r.localName === 'tr')
    .map((tr) =>
      Array.from(tr.children)
        .filter((c) => c.localName === 'tc')
        .map((tc) =>
          Array.from(tc.children)
            .filter((p) => p.localName === 'p')
            .map(paragraphText)
            .join(' ')
            .replace(/\|/g, '\\|')
            .replace(/\s+/g, ' ')
            .trim(),
        ),
    )
    .filter((r) => r.length > 0);
  if (rows.length === 0) {
    return '';
  }
  const width = Math.max(...rows.map((r) => r.length));
  const line = (r: string[]) => '| ' + Array.from({ length: width }, (_, i) => r[i] ?? '').join(' | ') + ' |';
  return [line(rows[0]), '| ' + Array(width).fill('---').join(' | ') + ' |', ...rows.slice(1).map(line)].join('\n');
}

function child(el: Element | null | undefined, name: string): Element | null {
  if (!el) {
    return null;
  }
  for (const c of Array.from(el.children)) {
    if (c.localName === name) {
      return c;
    }
  }
  return null;
}

function attr(el: Element | null, name: string): string {
  return el?.getAttributeNS(W, name) ?? el?.getAttribute('w:' + name) ?? '';
}

/** One entry of a zip archive as text, null when it is not there. */
export async function unzipEntry(data: ArrayBuffer, name: string): Promise<string | null> {
  const v = new DataView(data);
  // The end of central directory record: within the last 64 KB.
  let eocd = -1;
  for (let i = data.byteLength - 22; i >= Math.max(0, data.byteLength - 65557); i--) {
    if (v.getUint32(i, true) === 0x06054b50) {
      eocd = i;
      break;
    }
  }
  if (eocd < 0) {
    throw new Error('not a zip file');
  }
  const count = v.getUint16(eocd + 10, true);
  let p = v.getUint32(eocd + 16, true);
  const dec = new TextDecoder();
  for (let n = 0; n < count && v.getUint32(p, true) === 0x02014b50; n++) {
    const method = v.getUint16(p + 10, true);
    const size = v.getUint32(p + 20, true);
    const nameLen = v.getUint16(p + 28, true);
    const extraLen = v.getUint16(p + 30, true);
    const commentLen = v.getUint16(p + 32, true);
    const local = v.getUint32(p + 42, true);
    const entry = dec.decode(new Uint8Array(data, p + 46, nameLen));
    if (entry === name) {
      const start = local + 30 + v.getUint16(local + 26, true) + v.getUint16(local + 28, true);
      const bytes = new Uint8Array(data, start, size);
      if (method === 0) {
        return dec.decode(bytes);
      }
      if (method !== 8) {
        throw new Error('unsupported zip compression');
      }
      const stream = new Blob([bytes]).stream().pipeThrough(new DecompressionStream('deflate-raw'));
      return dec.decode(await new Response(stream).arrayBuffer());
    }
    p += 46 + nameLen + extraLen + commentLen;
  }
  return null;
}

// ---- PDF -----------------------------------------------------------------

/** A line of a PDF page: its text, size, and whether it is all monospace. */
export interface PdfLine {
  text: string;
  size: number;
  mono: boolean;
  /** A gap above it larger than a line: a new paragraph. */
  gap: boolean;
}

/** The text of a PDF, line by line, with pdf.js (loaded only now). */
export async function pdfToMarkdown(data: ArrayBuffer): Promise<string> {
  const pdfjs = await import('pdfjs-dist');
  // The worker runs in this page: no separate file to serve.
  const worker = await import('pdfjs-dist/build/pdf.worker.min.mjs');
  (globalThis as { pdfjsWorker?: unknown }).pdfjsWorker = worker;
  const doc = await pdfjs.getDocument({ data: new Uint8Array(data) }).promise;
  const lines: PdfLine[] = [];
  for (let n = 1; n <= doc.numPages; n++) {
    const page = await doc.getPage(n);
    const content = await page.getTextContent();
    const fonts = content.styles as Record<string, { fontFamily?: string }>;
    let current: { y: number; parts: string[]; size: number; mono: boolean; gap: boolean } | null = null;
    let lastY: number | null = null;
    for (const item of content.items as { str: string; transform: number[]; height: number; fontName: string; hasEOL?: boolean }[]) {
      const y = Math.round(item.transform[5]);
      if (!current || Math.abs(current.y - y) > 2) {
        if (current) {
          lines.push({ text: current.parts.join(''), size: current.size, mono: current.mono, gap: current.gap });
        }
        const height = Math.max(item.height, 1);
        current = { y, parts: [], size: height, mono: true, gap: lastY !== null && lastY - y > height * 1.8 };
        lastY = y;
      }
      if (item.str.trim()) {
        const family = (fonts[item.fontName]?.fontFamily ?? '') + ' ' + item.fontName;
        current.mono &&= family.includes('monospace') || MONO.test(family);
        current.size = Math.max(current.size, item.height);
      }
      current.parts.push(item.str);
    }
    if (current) {
      lines.push({ text: current.parts.join(''), size: current.size, mono: current.mono, gap: true });
    }
  }
  return pdfLinesToMarkdown(lines);
}

/**
 * PDF lines as Markdown: monospace lines become code blocks, lines in a
 * clearly larger size become headings, the rest paragraphs (lines joined).
 */
export function pdfLinesToMarkdown(lines: PdfLine[]): string {
  const text = lines.filter((l) => l.text.trim() && !l.mono);
  const sizes = text.map((l) => Math.round(l.size)).sort((a, b) => a - b);
  const body = sizes[Math.floor(sizes.length / 2)] ?? 10;
  const out: string[] = [];
  let para: string[] = [];
  let code: string[] | null = null;
  const flush = () => {
    if (para.length) {
      out.push(para.join(' ').replace(/\s+/g, ' ').trim());
      para = [];
    }
    if (code) {
      out.push('```\n' + code.join('\n').replace(/\n+$/, '') + '\n```');
      code = null;
    }
  };
  for (const l of lines) {
    if (!l.text.trim()) {
      continue;
    }
    if (l.mono) {
      if (para.length) {
        out.push(para.join(' ').replace(/\s+/g, ' ').trim());
        para = [];
      }
      (code ??= []).push(l.text.replace(/\s+$/, ''));
      continue;
    }
    if (code) {
      flush();
    }
    const level = l.size >= body * 1.6 ? 1 : l.size >= body * 1.3 ? 2 : l.size >= body * 1.12 ? 3 : 0;
    if (level) {
      flush();
      out.push('#'.repeat(level) + ' ' + l.text.trim());
      continue;
    }
    if (l.gap) {
      flush();
    }
    if (/^\s*(?:[-•▪◦*]|\d+[.)])\s+/.test(l.text)) {
      flush();
      out.push('- ' + l.text.replace(/^\s*(?:[-•▪◦*]|\d+[.)])\s+/, '').trim());
      continue;
    }
    para.push(l.text.trim());
  }
  flush();
  return out.join('\n\n').replace(/^(- .*)\n\n(?=- )/gm, '$1\n');
}
