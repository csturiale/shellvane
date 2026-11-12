import { documentXmlToMarkdown, pdfLinesToMarkdown, unzipEntry } from './document-import';

const W = 'xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"';
const p = (text: string, style = '', font = '') =>
  `<w:p>${style ? `<w:pPr><w:pStyle w:val="${style}"/></w:pPr>` : ''}<w:r>${font ? `<w:rPr><w:rFonts w:ascii="${font}"/></w:rPr>` : ''}<w:t xml:space="preserve">${text}</w:t></w:r></w:p>`;

describe('document import', () => {
  it('reads a Word body: headings, lists, code styles, monospace runs, tables', () => {
    const xml =
      `<w:document ${W}><w:body>` +
      p('Restart ArgoCD', 'Titolo1') +
      p('Check the pods first.') +
      `<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/></w:numPr></w:pPr><w:r><w:t>Log in</w:t></w:r></w:p>` +
      `<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/></w:numPr></w:pPr><w:r><w:t>Switch context</w:t></w:r></w:p>` +
      p('kubectl -n argocd get pods', 'Code') +
      p('kubectl -n argocd rollout restart deploy/argocd-server', '', 'Consolas') +
      p('Steps', 'Heading2') +
      `<w:tbl><w:tr><w:tc><w:p><w:r><w:t>Env</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>Context</w:t></w:r></w:p></w:tc></w:tr>` +
      `<w:tr><w:tc><w:p><w:r><w:t>prd</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>shop-prd</w:t></w:r></w:p></w:tc></w:tr></w:tbl>` +
      `</w:body></w:document>`;
    expect(documentXmlToMarkdown(xml)).toBe(
      [
        '# Restart ArgoCD',
        'Check the pods first.',
        '- Log in\n- Switch context',
        '```\nkubectl -n argocd get pods\nkubectl -n argocd rollout restart deploy/argocd-server\n```',
        '## Steps',
        '| Env | Context |\n| --- | --- |\n| prd | shop-prd |',
      ].join('\n\n'),
    );
  });

  it('reads a stored entry of a zip and reports a missing one', async () => {
    const enc = new TextEncoder();
    const name = enc.encode('word/document.xml');
    const body = enc.encode('<w:document/>');
    const local = new Uint8Array(30 + name.length + body.length);
    const lv = new DataView(local.buffer);
    lv.setUint32(0, 0x04034b50, true);
    lv.setUint32(18, body.length, true);
    lv.setUint32(22, body.length, true);
    lv.setUint16(26, name.length, true);
    local.set(name, 30);
    local.set(body, 30 + name.length);
    const central = new Uint8Array(46 + name.length);
    const cv = new DataView(central.buffer);
    cv.setUint32(0, 0x02014b50, true);
    cv.setUint32(20, body.length, true);
    cv.setUint32(24, body.length, true);
    cv.setUint16(28, name.length, true);
    cv.setUint32(42, 0, true);
    central.set(name, 46);
    const end = new Uint8Array(22);
    const ev = new DataView(end.buffer);
    ev.setUint32(0, 0x06054b50, true);
    ev.setUint16(8, 1, true);
    ev.setUint16(10, 1, true);
    ev.setUint32(12, central.length, true);
    ev.setUint32(16, local.length, true);
    const zip = new Uint8Array(local.length + central.length + end.length);
    zip.set(local, 0);
    zip.set(central, local.length);
    zip.set(end, local.length + central.length);
    expect(await unzipEntry(zip.buffer, 'word/document.xml')).toBe('<w:document/>');
    expect(await unzipEntry(zip.buffer, 'word/other.xml')).toBeNull();
    await expect(unzipEntry(new Uint8Array(40).buffer, 'x')).rejects.toThrow('not a zip file');
  });

  it('turns PDF lines into headings, paragraphs, lists and code blocks', () => {
    const l = (text: string, size = 10, mono = false, gap = false) => ({ text, size, mono, gap });
    expect(
      pdfLinesToMarkdown([
        l('Restart ArgoCD', 18),
        l('Check the pods', 10, false, true),
        l('before restarting.'),
        l('kubectl get pods', 9, true),
        l('kubectl rollout restart deploy/x', 9, true),
        l('• Then watch the logs', 10, false, true),
        l('Notes', 13, false, true),
        l('Done.', 10, false, true),
      ]),
    ).toBe(['# Restart ArgoCD', 'Check the pods before restarting.', '```\nkubectl get pods\nkubectl rollout restart deploy/x\n```', '- Then watch the logs', '## Notes', 'Done.'].join('\n\n'));
  });
});
