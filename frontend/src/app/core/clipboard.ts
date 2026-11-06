import { Clipboard } from '@wailsio/runtime';

/**
 * System clipboard. Both the Wails clipboard (the desktop webview may refuse
 * navigator.clipboard) and the web API are used, so it works in the desktop
 * app and in the browser build alike.
 */
export async function writeClipboard(text: string): Promise<void> {
  const results = await Promise.allSettled([Clipboard.SetText(text), navigator.clipboard.writeText(text)]);
  if (results.every((r) => r.status === 'rejected')) {
    throw (results[0] as PromiseRejectedResult).reason;
  }
}

export async function readClipboard(): Promise<string> {
  if (inDesktopWebview()) {
    // In the desktop app the system clipboard is read through Wails: on
    // macOS navigator.clipboard.readText() makes WebKit show its "Paste"
    // callout instead of pasting.
    try {
      return await withTimeout(Clipboard.Text());
    } catch {
      // Fall back to the web API below.
    }
  }
  try {
    return await navigator.clipboard.readText();
  } catch {
    // The webview refused: ask Wails, without waiting forever.
    return withTimeout(Clipboard.Text());
  }
}

function withTimeout(p: Promise<string>): Promise<string> {
  return Promise.race([p, new Promise<string>((resolve) => setTimeout(() => resolve(''), READ_TIMEOUT_MS))]);
}

/** Tells whether the page runs in the desktop app's webview (not a browser). */
export function inDesktopWebview(): boolean {
  const w = window as unknown as { webkit?: { messageHandlers?: unknown }; chrome?: { webview?: unknown } };
  return !!(w.webkit?.messageHandlers || w.chrome?.webview);
}

const READ_TIMEOUT_MS = 2000;
