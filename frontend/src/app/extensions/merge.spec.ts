import { mergeTranslations } from './merge';

describe('mergeTranslations', () => {
  it('merges nested keys without dropping the base ones', () => {
    const base = { errors: { a: 'A' }, x: 'X' };
    expect(mergeTranslations(base, { errors: { b: 'B' }, y: 'Y' })).toEqual({ errors: { a: 'A', b: 'B' }, x: 'X', y: 'Y' });
    expect(base).toEqual({ errors: { a: 'A' }, x: 'X' });
  });
});
