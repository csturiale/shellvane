import { GUIDE_TOPICS, excerpt, parseAction, search, sections } from './guide';
import { render } from './guide-render';
import { expandKeys } from './guide.store';

const md = `# Authentication {#top}

Intro text.

## Two-factor codes (TOTP) {#totp}

Paste the secret in the [host](shellvane:topic/hosts#manage) field. Città accentate.

## Agent forwarding {#forwarding}

Asks before every signature. See [docs](https://example.com).
`;

describe('guide', () => {
  it('splits topics into sections with stable anchors', () => {
    const s = sections('auth', md);
    expect(s.map((x) => x.anchor)).toEqual(['top', 'totp', 'forwarding']);
    expect(s[1].title).toBe('Two-factor codes (TOTP)');
    expect(s[1].text).toContain('Paste the secret in the host field');
  });

  it('searches titles first, ignoring case and accents', () => {
    const all = sections('auth', md);
    expect(search(all, 'totp')[0].anchor).toBe('totp');
    expect(search(all, 'CITTA')[0].anchor).toBe('totp');
    expect(search(all, 'signature').map((s) => s.anchor)).toEqual(['forwarding']);
    expect(search(all, '')).toEqual([]);
    expect(excerpt(search(all, 'signature')[0], 'signature')).toContain('signature');
  });

  it('renders anchors, in-app actions and external links', () => {
    const html = render(md);
    expect(html).toContain('<h2 id="totp">Two-factor codes (TOTP)</h2>');
    expect(html).toContain('class="action" data-action="topic/hosts#manage"');
    expect(html).toContain('href="https://example.com" target="_blank"');
    expect(html).not.toContain('{#');
  });

  it('parses actions', () => {
    expect(parseAction('topic/auth#totp')).toEqual({ kind: 'topic', target: 'auth', anchor: 'totp' });
    expect(parseAction('action/newLocal')).toEqual({ kind: 'action', target: 'newLocal', anchor: '' });
    expect(GUIDE_TOPICS).toContain('kubernetes');
  });
});

describe('expandKeys', () => {
  it('writes the shortcuts for this platform', () => {
    const text = expandKeys('Press [[K]], then [[alt]]+arrows.');
    expect(text).toMatch(/^Press (⌘K|Ctrl\+Shift\+K), then (⌥|Alt)\+arrows\.$/);
    expect(text).not.toContain('[[');
  });
});
