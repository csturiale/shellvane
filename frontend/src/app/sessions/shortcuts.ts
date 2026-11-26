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
  // Moving between panes: Alt+arrows, or ⌘⌥+arrows on macOS, where ⌥+arrows
  // move by word in the shell (see macLineEdit).
  if (ev.altKey && !ev.ctrlKey && !ev.shiftKey && ev.metaKey === isMac) {
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

/**
 * macOS text-editing keys, sent to the shell as readline/zle sequences (as
 * Terminal and iTerm2 do): ⌥←/→ by word, ⌘←/→ to the start or end of the
 * line, ⌥⌫ deletes a word, ⌘⌫ the line before the cursor, ⌥⌦ the word
 * after it, ⌘⌦ the rest of the line. Null for any other key, and outside
 * macOS.
 */
export function macLineEdit(ev: KeyboardEvent, mac = isMac): string | null {
  if (!mac || ev.type !== 'keydown' || ev.ctrlKey || ev.shiftKey || ev.altKey === ev.metaKey) {
    return null;
  }
  const word = ev.altKey;
  switch (ev.key) {
    case 'ArrowLeft':
      return word ? '\x1bb' : '\x01';
    case 'ArrowRight':
      return word ? '\x1bf' : '\x05';
    case 'Backspace':
      return word ? '\x1b\x7f' : '\x15';
    case 'Delete':
      return word ? '\x1bd' : '\x0b';
  }
  return null;
}
