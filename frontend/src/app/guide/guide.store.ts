import { Injectable, inject, signal } from '@angular/core';
import { TranslocoService } from '@jsverse/transloco';

import { altKey, combo } from '../sessions/shortcuts';
import { ALL_GUIDE_TOPICS, GuideSection, sections } from './guide';

/**
 * Writes the shortcuts of the guide ([[K]], [[alt]]) in the notation of
 * this platform: ⌘K on macOS, Ctrl+Shift+K elsewhere.
 */
export function expandKeys(markdown: string): string {
  return markdown.replace(/\[\[([^\]\n]+)\]\]/g, (_, key: string) => (key === 'alt' ? altKey : combo(key)));
}

/** Loads the guide of the current language (bundled with the app). */
@Injectable({ providedIn: 'root' })
export class GuideStore {
  private readonly transloco = inject(TranslocoService);
  private readonly cache = new Map<string, string>();
  /** Every section of every topic, for the search. */
  readonly index = signal<GuideSection[]>([]);
  private indexed = '';

  /** Markdown of a topic in the active language (English if missing). */
  async topic(id: string): Promise<string> {
    const lang = this.transloco.getActiveLang();
    const key = `${lang}/${id}`;
    const cached = this.cache.get(key);
    if (cached !== undefined) {
      return cached;
    }
    let text = await this.fetch(`guide/${lang}/${id}.md`);
    if (text === null && lang !== 'en') {
      text = await this.fetch(`guide/en/${id}.md`);
    }
    const md = expandKeys(text ?? '');
    this.cache.set(key, md);
    return md;
  }

  /** Builds the search index for the active language, once. */
  async loadIndex(): Promise<void> {
    const lang = this.transloco.getActiveLang();
    if (this.indexed === lang) {
      return;
    }
    const all = await Promise.all(ALL_GUIDE_TOPICS.map(async (id) => sections(id, await this.topic(id))));
    this.index.set(all.flat());
    this.indexed = lang;
  }

  private async fetch(url: string): Promise<string | null> {
    try {
      const res = await fetch(url);
      return res.ok ? await res.text() : null;
    } catch {
      return null;
    }
  }
}
