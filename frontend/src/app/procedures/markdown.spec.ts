import { commandOf, docLink, fill, placeholders, renderProcedure } from './markdown';

const labels = { paste: 'Paste', run: 'Run', copy: 'Copy', done: 'Done' };

describe('procedure markdown', () => {
  it('finds the commands and keeps HTML out', () => {
    const md = [
      '# Restore',
      '<script>alert(1)</script> <img src=x onerror=alert(1)>',
      '1. Stop the app:',
      '   ```bash',
      '   kubectl -n <namespace> scale deploy/web --replicas=0',
      '   ```',
      '2. Check:',
      '   ```console',
      '   $ kubectl get pods',
      '   No resources found.',
      '   ```',
      '```yaml',
      'replicas: 0',
      '```',
      '![diagram](x.png) [next](../db/after.md) [site](https://example.com) [bad](javascript:alert(1))',
    ].join('\n');
    const p = renderProcedure(md, 'ops/restore.md', labels);
    expect(p.blocks.map((b) => [b.lang, b.shell, b.command])).toEqual([
      ['bash', true, 'kubectl -n <namespace> scale deploy/web --replicas=0'],
      ['console', true, 'kubectl get pods'],
      ['yaml', false, 'replicas: 0'],
    ]);
    expect(p.html).not.toContain('<script');
    expect(p.html).not.toContain('<img');
    expect(p.html).not.toContain('javascript:');
    expect(p.html).toContain('data-doc="db/after.md"');
    expect(p.html).toContain('href="https://example.com"');
    expect(p.html).toContain('data-act="copy" data-i="2"');
  });

  it('strips prompts only from transcripts', () => {
    expect(commandOf('$ ls\n$ pwd\n', 'bash')).toBe('ls\npwd');
    expect(commandOf('echo $HOME\n', 'bash')).toBe('echo $HOME');
    expect(commandOf('$ ls\nfile.txt', 'sh')).toBe('$ ls\nfile.txt');
    expect(commandOf('PS C:\\> Get-Service\nStatus Name', 'console')).toBe('Get-Service');
  });

  it('fills placeholders', () => {
    const cmd = 'gcloud container clusters get-credentials {{cluster}} --project <project> < input.txt';
    expect(placeholders(cmd)).toEqual(['cluster', 'project']);
    expect(fill(cmd, { cluster: 'prod', project: '' })).toBe('gcloud container clusters get-credentials prod --project <project> < input.txt');
  });

  it('resolves links between procedures', () => {
    expect(docLink('./b.md#step', 'a/x.md')).toBe('a/b.md');
    expect(docLink('../c/d.markdown', 'a/x.md')).toBe('c/d.markdown');
    expect(docLink('https://x/y.md', 'a/x.md')).toBeNull();
    expect(docLink('img.png', 'a/x.md')).toBeNull();
  });
});
