import { SCHEMES, contrast, findScheme } from './schemes';

describe('terminal schemes', () => {
  it('have unique ids and a full 16-color palette', () => {
    expect(new Set(SCHEMES.map((s) => s.id)).size).toBe(SCHEMES.length);
    for (const s of SCHEMES) {
      expect(s.colors.brightWhite, s.id).toMatch(/^#[0-9a-f]{6}$/);
      expect(s.colors.background, s.id).toMatch(/^#[0-9a-f]{6}$/);
    }
  });

  it('keep the Shellvane palettes readable on their background', () => {
    // Text colors (not black/white, which double as backgrounds) reach 3:1.
    const text = ['red', 'green', 'yellow', 'blue', 'magenta', 'cyan'] as const;
    for (const id of ['shellvane-light', 'shellvane-dark']) {
      const s = findScheme(id, false);
      for (const c of text) {
        expect(contrast(s.colors[c] as string, s.colors.background as string), `${id} ${c}`).toBeGreaterThanOrEqual(3);
      }
      expect(contrast(s.colors.foreground as string, s.colors.background as string)).toBeGreaterThanOrEqual(7);
    }
  });

  it('follows the app theme in auto mode', () => {
    expect(findScheme('auto', true).id).toBe('shellvane-dark');
    expect(findScheme(null, false).id).toBe('shellvane-light');
    expect(findScheme('dracula', false).id).toBe('dracula');
    expect(findScheme('missing', true).id).toBe('shellvane-dark');
  });
});
