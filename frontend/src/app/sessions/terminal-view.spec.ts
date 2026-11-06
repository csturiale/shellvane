import { cwdOf } from './terminal-view';

describe('cwdOf', () => {
  it('reads OSC 7 reports', () => {
    expect(cwdOf('file://web-1/home/deploy/my%20app')).toBe('/home/deploy/my app');
    expect(cwdOf('file:///srv')).toBe('/srv');
    expect(cwdOf('http://x/y')).toBe('');
  });
});
