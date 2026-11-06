import { Window } from '@wailsio/runtime';

/** The token of a window that shows a tab moved out of the main window. */
export function detachedToken(): string | null {
  try {
    return new URLSearchParams(window.location.search).get('detached');
  } catch {
    return null;
  }
}

/** Closes this window (the native one in the app, the tab in a browser). */
export function closeWindow(): void {
  Window.Close().catch(() => window.close());
}
