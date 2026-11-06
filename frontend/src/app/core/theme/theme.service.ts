import { Injectable, computed, signal } from '@angular/core';

import { Window } from '@wailsio/runtime';

import { readPreference, writePreference } from '../preferences';
import { inDesktopWebview } from '../clipboard';

/** Sizes offered for the whole interface, in percent. */
export const UI_SCALES = [80, 90, 100, 110, 120, 135, 150] as const;
const SCALE_KEY = 'ui.scale';

export const THEME_MODES = ['system', 'light', 'dark'] as const;
export type ThemeMode = (typeof THEME_MODES)[number];

const STORAGE_KEY = 'theme';

@Injectable({ providedIn: 'root' })
export class ThemeService {
  readonly modes = THEME_MODES;
  readonly mode = signal<ThemeMode>(this.initial());
  private readonly systemDark = signal(matchesDark());
  /** Whether the app is currently dark, resolving "system". */
  readonly isDark = computed(() => (this.mode() === 'system' ? this.systemDark() : this.mode() === 'dark'));

  constructor() {
    try {
      window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => this.systemDark.set(e.matches));
    } catch {
      // No matchMedia (tests).
    }
  }

  /**
   * Size of the whole interface in percent. The terminals keep their own
   * font size: they divide it by this scale.
   */
  readonly scale = signal(initialScale());

  init(): void {
    this.apply(this.mode());
    this.applyScale(this.scale());
  }

  setScale(percent: number): void {
    this.scale.set(percent);
    writePreference(SCALE_KEY, String(percent));
    this.applyScale(percent);
  }

  /**
   * The desktop webview zooms the page (it reflows, like a browser's zoom);
   * a browser gets CSS zoom.
   */
  private applyScale(percent: number): void {
    const zoom = percent / 100;
    if (inDesktopWebview()) {
      Window.SetZoom(zoom).catch(() => this.cssZoom(zoom));
    } else {
      this.cssZoom(zoom);
    }
  }

  private cssZoom(zoom: number): void {
    (document.documentElement.style as CSSStyleDeclaration & { zoom: string }).zoom = zoom === 1 ? '' : String(zoom);
  }

  set(mode: ThemeMode): void {
    this.mode.set(mode);
    writePreference(STORAGE_KEY, mode);
    this.apply(mode);
  }

  private apply(mode: ThemeMode): void {
    const root = document.documentElement;
    if (mode === 'system') {
      root.removeAttribute('data-theme');
    } else {
      root.setAttribute('data-theme', mode);
    }
  }

  private initial(): ThemeMode {
    const stored = readPreference(STORAGE_KEY);
    return THEME_MODES.includes(stored as ThemeMode) ? (stored as ThemeMode) : 'system';
  }
}

function initialScale(): number {
  const n = Number(readPreference(SCALE_KEY));
  return (UI_SCALES as readonly number[]).includes(n) ? n : 100;
}

function matchesDark(): boolean {
  try {
    return window.matchMedia('(prefers-color-scheme: dark)').matches;
  } catch {
    return false;
  }
}
