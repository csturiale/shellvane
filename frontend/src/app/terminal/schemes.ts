import { computed, signal } from '@angular/core';
import type { ITheme } from '@xterm/xterm';

export interface Scheme {
  id: string;
  name: string;
  dark: boolean;
  colors: ITheme;
}

function palette(dark: boolean, base: Omit<ITheme, 'selectionBackground'>, ansi: string[]): ITheme {
  const names = [
    'black', 'red', 'green', 'yellow', 'blue', 'magenta', 'cyan', 'white',
    'brightBlack', 'brightRed', 'brightGreen', 'brightYellow', 'brightBlue', 'brightMagenta', 'brightCyan', 'brightWhite',
  ] as const;
  const theme: ITheme = { ...base, selectionBackground: dark ? 'rgba(255,255,255,0.22)' : 'rgba(0,0,0,0.16)' };
  names.forEach((n, i) => (theme[n] = ansi[i]));
  return theme;
}

/**
 * Built-in schemes. The Shellvane ones follow the app theme and keep every ANSI
 * color readable on their background (the default xterm palette does not on
 * light backgrounds).
 */
export const SCHEMES: Scheme[] = [
  {
    id: 'shellvane-light',
    name: 'Shellvane Light',
    dark: false,
    colors: palette(false, { background: '#fafafa', foreground: '#18181b', cursor: '#2563eb' }, [
      '#18181b', '#c62828', '#2e7d32', '#8d6e00', '#1e5bd6', '#8e24aa', '#00796b', '#5f5f69',
      '#52525b', '#e53935', '#388e3c', '#a07800', '#2962ff', '#ab47bc', '#00897b', '#27272a',
    ]),
  },
  {
    id: 'shellvane-dark',
    name: 'Shellvane Dark',
    dark: true,
    colors: palette(true, { background: '#111113', foreground: '#e4e4e7', cursor: '#60a5fa' }, [
      '#27272a', '#f87171', '#4ade80', '#facc15', '#60a5fa', '#c084fc', '#2dd4bf', '#d4d4d8',
      '#71717a', '#fca5a5', '#86efac', '#fde047', '#93c5fd', '#d8b4fe', '#5eead4', '#fafafa',
    ]),
  },
  {
    id: 'solarized-light',
    name: 'Solarized Light',
    dark: false,
    colors: palette(false, { background: '#fdf6e3', foreground: '#586e75', cursor: '#586e75' }, [
      '#073642', '#dc322f', '#859900', '#b58900', '#268bd2', '#d33682', '#2aa198', '#eee8d5',
      '#002b36', '#cb4b16', '#586e75', '#657b83', '#839496', '#6c71c4', '#93a1a1', '#fdf6e3',
    ]),
  },
  {
    id: 'solarized-dark',
    name: 'Solarized Dark',
    dark: true,
    colors: palette(true, { background: '#002b36', foreground: '#839496', cursor: '#93a1a1' }, [
      '#073642', '#dc322f', '#859900', '#b58900', '#268bd2', '#d33682', '#2aa198', '#eee8d5',
      '#002b36', '#cb4b16', '#586e75', '#657b83', '#839496', '#6c71c4', '#93a1a1', '#fdf6e3',
    ]),
  },
  {
    id: 'github-light',
    name: 'GitHub Light',
    dark: false,
    colors: palette(false, { background: '#ffffff', foreground: '#24292f', cursor: '#0969da' }, [
      '#24292f', '#cf222e', '#116329', '#4d2d00', '#0969da', '#8250df', '#1b7c83', '#6e7781',
      '#57606a', '#a40e26', '#1a7f37', '#633c01', '#218bff', '#a475f9', '#3192aa', '#8c959f',
    ]),
  },
  {
    id: 'dracula',
    name: 'Dracula',
    dark: true,
    colors: palette(true, { background: '#282a36', foreground: '#f8f8f2', cursor: '#f8f8f2' }, [
      '#21222c', '#ff5555', '#50fa7b', '#f1fa8c', '#bd93f9', '#ff79c6', '#8be9fd', '#f8f8f2',
      '#6272a4', '#ff6e6e', '#69ff94', '#ffffa5', '#d6acff', '#ff92df', '#a4ffff', '#ffffff',
    ]),
  },
  {
    id: 'nord',
    name: 'Nord',
    dark: true,
    colors: palette(true, { background: '#2e3440', foreground: '#d8dee9', cursor: '#d8dee9' }, [
      '#3b4252', '#bf616a', '#a3be8c', '#ebcb8b', '#81a1c1', '#b48ead', '#88c0d0', '#e5e9f0',
      '#4c566a', '#bf616a', '#a3be8c', '#ebcb8b', '#81a1c1', '#b48ead', '#8fbcbb', '#eceff4',
    ]),
  },
  {
    id: 'gruvbox-dark',
    name: 'Gruvbox Dark',
    dark: true,
    colors: palette(true, { background: '#282828', foreground: '#ebdbb2', cursor: '#ebdbb2' }, [
      '#282828', '#cc241d', '#98971a', '#d79921', '#458588', '#b16286', '#689d6a', '#a89984',
      '#928374', '#fb4934', '#b8bb26', '#fabd2f', '#83a598', '#d3869b', '#8ec07c', '#ebdbb2',
    ]),
  },
  {
    id: 'one-dark',
    name: 'One Dark',
    dark: true,
    colors: palette(true, { background: '#282c34', foreground: '#abb2bf', cursor: '#528bff' }, [
      '#282c34', '#e06c75', '#98c379', '#e5c07b', '#61afef', '#c678dd', '#56b6c2', '#abb2bf',
      '#5c6370', '#e06c75', '#98c379', '#e5c07b', '#61afef', '#c678dd', '#56b6c2', '#ffffff',
    ]),
  },
  {
    id: 'tokyo-night',
    name: 'Tokyo Night',
    dark: true,
    colors: palette(true, { background: '#1a1b26', foreground: '#c0caf5', cursor: '#c0caf5' }, [
      '#15161e', '#f7768e', '#9ece6a', '#e0af68', '#7aa2f7', '#bb9af7', '#7dcfff', '#a9b1d6',
      '#414868', '#f7768e', '#9ece6a', '#e0af68', '#7aa2f7', '#bb9af7', '#7dcfff', '#c0caf5',
    ]),
  },
  {
    id: 'monokai',
    name: 'Monokai',
    dark: true,
    colors: palette(true, { background: '#272822', foreground: '#f8f8f2', cursor: '#f8f8f0' }, [
      '#272822', '#f92672', '#a6e22e', '#f4bf75', '#66d9ef', '#ae81ff', '#a1efe4', '#f8f8f2',
      '#75715e', '#f92672', '#a6e22e', '#f4bf75', '#66d9ef', '#ae81ff', '#a1efe4', '#f9f8f5',
    ]),
  },
];

/** "auto" follows the app theme with the Shellvane schemes. */
export const AUTO_SCHEME = 'auto';

/** Schemes imported or made by the user (loaded by ThemesStore). */
export const customSchemes = signal<Scheme[]>([]);

/** Built-in schemes, then the user's. */
export const allSchemes = computed(() => [...SCHEMES, ...customSchemes()]);

export function findScheme(id: string | null | undefined, appIsDark: boolean): Scheme {
  const auto = SCHEMES[appIsDark ? 1 : 0];
  if (!id || id === AUTO_SCHEME) {
    return auto;
  }
  return allSchemes().find((s) => s.id === id) ?? auto;
}

/** WCAG contrast ratio between two #rrggbb colors. */
export function contrast(a: string, b: string): number {
  const lum = (hex: string) => {
    const [r, g, bl] = [1, 3, 5].map((i) => {
      const c = parseInt(hex.slice(i, i + 2), 16) / 255;
      return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
    });
    return 0.2126 * r + 0.7152 * g + 0.0722 * bl;
  };
  const [x, y] = [lum(a), lum(b)].sort((m, n) => n - m);
  return (x + 0.05) / (y + 0.05);
}
