import { SCHEMES } from '../terminal/schemes';
import { appColors, toScheme, toTheme } from './themes.store';

describe('themes', () => {
  it('turns a scheme into an editable theme and back', () => {
    const dracula = SCHEMES.find((s) => s.id === 'dracula')!;
    const t = toTheme(dracula, 'Mine');
    expect(t.name).toBe('Mine');
    expect(t.colors['background']).toBe('#282a36');
    expect(t.colors['selectionBackground']).toBeUndefined(); // rgba is left to the default
    const back = toScheme({ ...t, id: 'custom-1' });
    expect(back.colors.background).toBe('#282a36');
    expect(back.colors.selectionBackground).toContain('rgba');
  });

  it('derives app colors from the scheme', () => {
    const c = appColors(SCHEMES.find((s) => s.id === 'nord')!);
    expect(c['--bg']).toBe(c['--terminal-bg']);
    expect(c['--surface']).toContain('color-mix');
  });
});
