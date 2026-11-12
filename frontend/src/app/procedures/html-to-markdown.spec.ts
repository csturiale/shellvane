import { bookmarklet, MARKER } from './bookmarklet';
import { htmlToMarkdown, pageContent } from './html-to-markdown';

const html = (s: string) => new DOMParser().parseFromString(s, 'text/html');

describe('htmlToMarkdown', () => {
  it('keeps headings, lists, tables and code blocks with their language', () => {
    const doc = html(`
      <nav>menu</nav>
      <div id="main-content" class="wiki-content">
        <h1>Restart <em>web</em></h1>
        <p>Use it when the <code>web</code> pods are <strong>down</strong>. See <a href="https://wiki.example.com/rb">rollback</a>.</p>
        <ol>
          <li>Pick the cluster:
            <div class="code panel"><div class="codeContent"><pre class="syntaxhighlighter-pre" data-syntaxhighlighter-params="brush: bash; gutter: false">gcloud container clusters get-credentials &lt;cluster&gt;</pre></div></div>
          </li>
          <li>Restart: <pre><code class="language-sh">kubectl -n web rollout restart deploy/web
kubectl -n web rollout status deploy/web</code></pre></li>
        </ol>
        <table><tr><th>Env</th><th>Cluster</th></tr><tr><td>prod</td><td>gke-prod</td></tr></table>
        <script>alert(1)</script><button>Edit</button>
      </div>`);
    const md = htmlToMarkdown(pageContent(doc));
    expect(md).toBe(
      [
        '# Restart _web_',
        '',
        'Use it when the `web` pods are **down** . See [rollback](https://wiki.example.com/rb).',
        '',
        '1. Pick the cluster:',
        '',
        '   ```bash',
        '   gcloud container clusters get-credentials <cluster>',
        '   ```',
        '2. Restart:',
        '',
        '   ```sh',
        '   kubectl -n web rollout restart deploy/web',
        '   kubectl -n web rollout status deploy/web',
        '   ```',
        '',
        '| Env | Cluster |',
        '| --- | --- |',
        '| prod | gke-prod |',
      ].join('\n'),
    );
  });

  it('runs on its own, as the bookmarklet does', () => {
    const fn = new Function('return ' + htmlToMarkdown.toString())() as typeof htmlToMarkdown;
    expect(fn(html('<pre>ls -la</pre>').body)).toBe('```\nls -la\n```');
    const code = decodeURIComponent(bookmarklet().slice('javascript:'.length));
    expect(() => new Function(code)).not.toThrow();
    expect(code).toContain(JSON.stringify(MARKER));
  });
});
