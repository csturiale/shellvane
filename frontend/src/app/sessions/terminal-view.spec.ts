import { joinFrames, cwdOf } from './terminal-view';

describe('cwdOf', () => {
  it('reads OSC 7 reports', () => {
    expect(cwdOf('file://web-1/home/deploy/my%20app')).toBe('/home/deploy/my app');
    expect(cwdOf('file:///srv')).toBe('/srv');
    expect(cwdOf('http://x/y')).toBe('');
  });
});

describe('joinFrames', () => {
  it('joins output frames in order', () => {
    const out = joinFrames([new Uint8Array([1, 2]), new Uint8Array([]), new Uint8Array([3])]);
    expect(Array.from(out)).toEqual([1, 2, 3]);
  });
});
