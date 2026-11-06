import { CommandHistory, LineTracker, keepable } from './suggest';

describe('LineTracker', () => {
  it('follows typing, backspace and Enter', () => {
    const t = new LineTracker();
    expect(t.feed('kubectl get po')).toEqual([]);
    t.feed('\x7f\x7fpods');
    expect(t.line).toBe('kubectl get pods');
    expect(t.feed('\r')).toEqual(['kubectl get pods']);
    expect(t.line).toBe('');
  });

  it('stops following a line edited with keys it does not model', () => {
    const t = new LineTracker();
    t.feed('git sta\t');
    expect(t.clean).toBe(false);
    expect(t.feed('\r')).toEqual([]);
    t.feed('ls\x1b[D\x1b[Ca');
    expect(t.feed('\r')).toEqual([]);
    expect(t.clean).toBe(true);
    t.feed('abc\x03');
    expect(t.line).toBe('');
    expect(t.clean).toBe(true);
  });

  it('handles several commands pasted at once', () => {
    expect(new LineTracker().feed('cd /tmp\rls -la\r')).toEqual(['cd /tmp', 'ls -la']);
  });
});

describe('keepable', () => {
  it('drops secrets and commands starting with a space', () => {
    expect(keepable('docker ps')).toBe(true);
    expect(keepable('mkdir -p /srv/app && ssh -p 2222 web')).toBe(true);
    expect(keepable(' export X=1')).toBe(false);
    expect(keepable('mysql -u root -psecret db')).toBe(false);
    expect(keepable('curl -H "Authorization: Bearer x"')).toBe(false);
    expect(keepable('export AWS_SECRET_ACCESS_KEY=abc')).toBe(false);
    expect(keepable('PASSWORD=x ./run')).toBe(false);
    expect(keepable('git push --token=abc')).toBe(false);
  });
});

describe('CommandHistory', () => {
  beforeEach(() => localStorage.clear());

  it('suggests this connection first, then others, then snippets', () => {
    const h = new CommandHistory();
    h.remember('host:a', 'systemctl status nginx');
    h.remember('host:b', 'systemctl restart nginx');
    h.remember('host:a', 'ls -la');
    expect(h.suggest('host:a', 'sys')).toBe('systemctl status nginx');
    expect(h.suggest('host:c', 'systemctl r')).toBe('systemctl restart nginx');
    expect(h.suggest('host:a', 'kubectl g', ['kubectl get pods -A'])).toBe('kubectl get pods -A');
    expect(h.suggest('host:a', 'ls -la')).toBe(''); // nothing longer
    expect(h.suggest('host:a', '')).toBe('');
  });

  it('keeps the newest once, survives a reload and clears', () => {
    const h = new CommandHistory();
    h.remember('s', 'echo 1');
    h.remember('s', 'echo 2');
    h.remember('s', 'echo 1');
    expect(new CommandHistory().suggest('s', 'echo ')).toBe('echo 1');
    h.remember('s', 'mysql -psecret');
    expect(h.size()).toBe(2);
    h.clear();
    expect(new CommandHistory().size()).toBe(0);
  });
});
