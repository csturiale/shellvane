import { clock, eventsUntil, parseCast } from './cast';

const cast = [
  JSON.stringify({ version: 2, width: 100, height: 30, title: 'web-1' }),
  JSON.stringify([0.5, 'o', 'hello ']),
  JSON.stringify([1.0, 'r', '120x40']),
  JSON.stringify([10.0, 'o', 'world']),
  '[11.0, "o", "trunc',
].join('\n');

describe('cast', () => {
  it('parses events and shortens idle time', () => {
    const c = parseCast(cast);
    expect(c.width).toBe(100);
    expect(c.title).toBe('web-1');
    expect(c.events.map((e) => e.kind)).toEqual(['o', 'r', 'o']);
    expect(c.events[2].time).toBe(3); // 9 s pause shortened to 2
    expect(c.duration).toBe(3);
    expect(eventsUntil(c.events, 1)).toBe(2);
    expect(clock(65.4)).toBe('1:05');
  });

  it('rejects other formats', () => {
    expect(() => parseCast('{"version":1}')).toThrow();
  });
});
