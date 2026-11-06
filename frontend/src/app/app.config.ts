import {
  ApplicationConfig,
  inject,
  isDevMode,
  provideAppInitializer,
  provideBrowserGlobalErrorListeners,
} from '@angular/core';
import { provideTransloco } from '@jsverse/transloco';
import { provideTranslocoMessageformat } from '@jsverse/transloco-messageformat';

import { BundledTranslocoLoader } from './core/i18n/transloco-loader';
import { AVAILABLE_LANGUAGES, SOURCE_LANGUAGE } from './core/i18n/languages';
import { LanguageService } from './core/i18n/language.service';
import { ThemeService } from './core/theme/theme.service';

export const appConfig: ApplicationConfig = {
  providers: [
    provideBrowserGlobalErrorListeners(),
    provideTransloco({
      config: {
        availableLangs: [...AVAILABLE_LANGUAGES],
        defaultLang: SOURCE_LANGUAGE,
        fallbackLang: SOURCE_LANGUAGE,
        missingHandler: { useFallbackTranslation: true, logMissingKey: isDevMode() },
        reRenderOnLangChange: true,
        prodMode: !isDevMode(),
      },
      loader: BundledTranslocoLoader,
    }),
    // ICU plurals and selects: {count, plural, one {...} other {...}}
    provideTranslocoMessageformat(),
    provideAppInitializer(() => {
      inject(ThemeService).init();
      inject(LanguageService).init();
    }),
  ],
};
