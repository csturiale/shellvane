import { Injectable, effect, inject, signal } from '@angular/core';

import { Theme, ThemeApi } from '../core/backend/theme-api';
import { ThemeService } from '../core/theme/theme.service';
import { Scheme, customSchemes, findScheme } from '../terminal/schemes';
import { TerminalPrefs } from '../terminal/terminal-prefs';

/** Color keys of a scheme, in the editor's order. */
export const COLOR_KEYS = [
  'background', 'foreground', 'cursor', 'cursorAccent', 'selectionBackground',
  'black', 'red', 'green', 'yellow', 'blue', 'magenta', 'cyan', 'white',
  'brightBlack', 'brightRed', 'brightGreen', 'brightYellow', 'brightBlue', 'brightMagenta', 'brightCyan', 'brightWhite',
] as const;

/** A stored theme as an xterm.js scheme. */
export function toScheme(t: Theme): Scheme {
  const colors: Record<string, string> = { ...t.colors };
  colors['selectionBackground'] ??= t.dark ? 'rgba(255,255,255,0.22)' : 'rgba(0,0,0,0.16)';
  return { id: t.id, name: t.name, dark: t.dark, colors };
}

/** A scheme as an editable theme (rgba selections become hex). */
export function toTheme(s: Scheme, name: string, id = ''): Theme {
  const colors: Record<string, string> = {};
  for (const k of COLOR_KEYS) {
    const v = (s.colors as Record<string, string | undefined>)[k];
    if (v && /^#[0-9a-f]{6}$/i.test(v)) {
      colors[k] = v.toLowerCase();
    }
  }
  return { id, name, dark: s.dark, colors };
}

/** App colors derived from a terminal scheme. */
export function appColors(s: Scheme): Record<string, string> {
  const bg = s.colors.background ?? '#000000';
  const fg = s.colors.foreground ?? '#ffffff';
  const mix = (pct: number) => `color-mix(in srgb, ${fg} ${pct}%, ${bg})`;
  return {
    '--bg': bg,
    '--surface': mix(5),
    '--border': mix(14),
    '--text': fg,
    '--text-muted': mix(62),
    '--accent': s.colors.blue ?? s.colors.cursor ?? fg,
    '--terminal-bg': bg,
    '--hover': `color-mix(in srgb, ${fg} 8%, transparent)`,
  };
}

/** Custom schemes, and app colors that can follow the terminal scheme. */
@Injectable({ providedIn: 'root' })
export class ThemesStore {
  private readonly api = inject(ThemeApi);
  private readonly prefs = inject(TerminalPrefs);
  private readonly appTheme = inject(ThemeService);
  readonly themes = signal<Theme[]>([]);
  private applied: string[] = [];

  constructor() {
    effect(() => {
      const scheme = this.prefs.matchApp() && this.prefs.scheme() !== 'auto' ? findScheme(this.prefs.scheme(), this.appTheme.isDark()) : null;
      this.applyApp(scheme);
    });
  }

  async load(): Promise<void> {
    const list = await this.api.list();
    this.themes.set(list);
    customSchemes.set(list.map(toScheme));
  }

  async save(t: Theme): Promise<Theme> {
    const saved = await this.api.save(t);
    await this.load();
    return saved;
  }

  async delete(id: string): Promise<void> {
    await this.api.delete(id);
    await this.load();
    if (this.prefs.scheme() === id) {
      this.prefs.setScheme('auto');
    }
  }

  async import(fileName: string, content: string): Promise<Theme[]> {
    const out = await this.api.import(fileName, content);
    await this.load();
    return out;
  }

  export(t: Theme): Promise<string> {
    return this.api.export(t);
  }

  private applyApp(scheme: Scheme | null): void {
    const root = document.documentElement;
    for (const k of this.applied) {
      root.style.removeProperty(k);
    }
    this.applied = [];
    if (!scheme) {
      this.appTheme.init(); // back to the light/dark/system choice
      return;
    }
    root.setAttribute('data-theme', scheme.dark ? 'dark' : 'light');
    for (const [k, v] of Object.entries(appColors(scheme))) {
      root.style.setProperty(k, v);
      this.applied.push(k);
    }
  }
}
