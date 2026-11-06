/**
 * Workspace shortcuts. On macOS Cmd is used; elsewhere Ctrl+Shift, because
 * plain Ctrl+letter belongs to the remote shell.
 */
export type ShortcutAction =
  | 'splitRight'
  | 'splitDown'
  | 'closePane'
  | 'broadcast'
  | 'palette'
  | 'newLocal'
  | 'search'
  | 'maximize'
  | 'zoomIn'
  | 'zoomOut'
  | 'zoomReset'
  | 'cheatsheet'
  | 'togglePanel'
  | 'focusLeft'
  | 'focusRight'
  | 'focusUp'
  | 'focusDown';

export const isMac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform);

/** The app modifier plus key, in the notation of this platform. */
export function combo(key: string): string {
  return (isMac ? '⌘' : 'Ctrl+Shift+') + key;
}

/** The Alt key's name on this platform. */
export const altKey = isMac ? '⌥' : 'Alt';

export function shortcutAction(ev: KeyboardEvent): ShortcutAction | null {
  if (ev.type !== 'keydown') {
    return null;
  }
  const mod = isMac ? ev.metaKey && !ev.ctrlKey : ev.ctrlKey && ev.shiftKey && !ev.metaKey;
  if (mod && !ev.altKey) {
    switch (ev.key.toLowerCase()) {
      case 'd':
        return 'splitRight';
      case 'e':
        return 'splitDown';
      case 'w':
        return 'closePane';
      case 'b':
        return 'broadcast';
      case 'k':
        return 'palette';
      case 't':
        return 'newLocal';
      case 'f':
        return 'search';
      case 'enter':
        return 'maximize';
      case '=':
      case '+':
        return 'zoomIn';
      case '-':
      case '_':
        return 'zoomOut';
      case '0':
      case ')':
        return 'zoomReset';
      case '/':
      case '?':
        return 'cheatsheet';
      case 'l':
        return 'togglePanel';
    }
  }
  if (ev.altKey && !ev.ctrlKey && !ev.metaKey && !ev.shiftKey) {
    switch (ev.key) {
      case 'ArrowLeft':
        return 'focusLeft';
      case 'ArrowRight':
        return 'focusRight';
      case 'ArrowUp':
        return 'focusUp';
      case 'ArrowDown':
        return 'focusDown';
    }
  }
  return null;
}

/**
 * Terminal copy and paste: Ctrl+Shift+C/V (Ctrl+C is the interrupt) or
 * Cmd+C/V on macOS.
 */
export function clipboardAction(ev: KeyboardEvent): 'copy' | 'paste' | null {
  if (ev.type !== 'keydown' || ev.altKey) {
    return null;
  }
  const mod = isMac ? ev.metaKey && !ev.ctrlKey && !ev.shiftKey : ev.ctrlKey && ev.shiftKey && !ev.metaKey;
  if (!mod) {
    return null;
  }
  switch (ev.key.toLowerCase()) {
    case 'c':
      return 'copy';
    case 'v':
      return 'paste';
  }
  return null;
}

export function isAppShortcut(ev: KeyboardEvent): boolean {
  return shortcutAction(ev) !== null;
}
