export const SOURCE_LANGUAGE = 'en';
export const AVAILABLE_LANGUAGES = ['en', 'it'] as const;
export type Language = (typeof AVAILABLE_LANGUAGES)[number];

export function isLanguage(value: unknown): value is Language {
  return AVAILABLE_LANGUAGES.includes(value as Language);
}

/** Picks the first supported language from BCP 47 tags such as "it-IT". */
export function matchLanguage(preferred: readonly string[]): Language {
  for (const tag of preferred) {
    const base = tag.toLowerCase().split('-')[0];
    if (isLanguage(base)) {
      return base;
    }
  }
  return SOURCE_LANGUAGE;
}
