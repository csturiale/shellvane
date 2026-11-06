import { Browser } from '@wailsio/runtime';

/** Whether a link may leave the app for the system browser: web pages only. */
export function isWebUrl(url: string): boolean {
  return /^https?:\/\//i.test(url.trim());
}

/**
 * Keeps the app's window on the app: a click on a link to a web page opens
 * it in the system browser, and links with other schemes (file:, data:,
 * javascript:…) do nothing. Links handled by the app (guide actions) call
 * preventDefault first and are left alone.
 */
export function guardLinks(doc: Document): void {
  doc.addEventListener('click', (ev) => {
    const a = (ev.target as Element | null)?.closest?.('a[href]') as HTMLAnchorElement | null;
    if (!a || ev.defaultPrevented) {
      return;
    }
    const href = a.getAttribute('href') ?? '';
    if (href.startsWith('#') || href === '') {
      return;
    }
    ev.preventDefault();
    if (isWebUrl(a.href)) {
      void openUrl(a.href);
    }
  });
}

/** Opens a URL in the system browser (a new tab outside the desktop app). */
export async function openUrl(url: string): Promise<void> {
  if (!isWebUrl(url)) {
    return; // only web pages leave the app
  }
  try {
    await Browser.OpenURL(url);
  } catch {
    window.open(url, '_blank', 'noopener');
  }
}
