import { fuzzyFilter, fuzzyScore } from './fuzzy';

describe('fuzzy', () => {
  it('requires characters in order', () => {
    expect(fuzzyScore('wp', 'web-prod')).not.toBeNull();
    expect(fuzzyScore('pw', 'web')).toBeNull();
    expect(fuzzyScore('', 'anything')).toBe(0);
  });

  it('ranks word starts and runs higher', () => {
    // web-prod: both letters start words; wrap: one does; swap: none.
    expect(fuzzyFilter('wp', ['swap', 'web-prod', 'wrap'], (s) => s)).toEqual(['web-prod', 'wrap', 'swap']);
    expect(fuzzyFilter('db', ['admin-box', 'db-1', 'my db'], (s) => s)[0]).toBe('db-1');
  });

  it('matches addresses', () => {
    expect(fuzzyFilter('10.0.2', ['web 10.0.1.5', 'db 10.0.2.15'], (s) => s)).toEqual(['db 10.0.2.15']);
  });
});
