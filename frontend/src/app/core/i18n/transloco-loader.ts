import { Injectable } from '@angular/core';
import { Translation, TranslocoLoader } from '@jsverse/transloco';

import { EXTENSIONS } from '../../extensions/extensions';
import { mergeTranslations } from '../../extensions/merge';

/**
 * Loads translations from the bundle instead of over HTTP: each language
 * becomes a lazy chunk, so the app works offline and from any asset server.
 */
@Injectable({ providedIn: 'root' })
export class BundledTranslocoLoader implements TranslocoLoader {
  async getTranslation(lang: string): Promise<Translation> {
    const base: Translation = (await import(`../../../i18n/${lang}.json`)).default;
    // Extensions bring their own strings, merged over the app's.
    const extra = await Promise.all(EXTENSIONS.flatMap((e) => (e.translations ? [e.translations(lang).catch(() => ({}))] : [])));
    return extra.reduce(mergeTranslations, base);
  }
}
