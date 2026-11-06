import { MenuItem, tidy } from './context-menu.service';

describe('tidy', () => {
  it('drops leading, trailing and repeated separators', () => {
    const a: MenuItem = { label: 'a', action: () => undefined };
    const b: MenuItem = { label: 'b', action: () => undefined };
    expect(tidy([null, a, null, null, b, null])).toEqual([a, null, b]);
    expect(tidy([null, null])).toEqual([]);
  });
});
