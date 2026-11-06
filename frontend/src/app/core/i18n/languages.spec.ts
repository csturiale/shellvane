import { matchLanguage } from './languages';

describe('matchLanguage', () => {
  it('matches on the base language', () => {
    expect(matchLanguage(['it-IT', 'en-US'])).toBe('it');
  });

  it('skips unsupported languages', () => {
    expect(matchLanguage(['ja-JP', 'it'])).toBe('it');
  });

  it('falls back to the source language', () => {
    expect(matchLanguage(['ja-JP'])).toBe('en');
    expect(matchLanguage([])).toBe('en');
  });
});
