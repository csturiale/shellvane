import { Injectable, inject, signal } from '@angular/core';
import { TranslocoService } from '@jsverse/transloco';

import { BackendService } from '../backend/backend.service';
import { readPreference, writePreference } from '../preferences';
import { AVAILABLE_LANGUAGES, Language, isLanguage, matchLanguage } from './languages';

const STORAGE_KEY = 'language';

@Injectable({ providedIn: 'root' })
export class LanguageService {
  private readonly transloco = inject(TranslocoService);
  private readonly backend = inject(BackendService);

  readonly available = AVAILABLE_LANGUAGES;
  readonly current = signal<Language>(this.initial());

  /** Applies the initial language; called once at startup. */
  init(): void {
    this.apply(this.current());
  }

  set(lang: Language): void {
    this.current.set(lang);
    writePreference(STORAGE_KEY, lang);
    this.apply(lang);
  }

  private apply(lang: Language): void {
    this.transloco.setActiveLang(lang);
    document.documentElement.lang = lang;
    // Keep Go-rendered strings (menus, tray) in the same language.
    void this.backend.setLanguage(lang);
  }

  private initial(): Language {
    const stored = readPreference(STORAGE_KEY);
    return isLanguage(stored) ? stored : matchLanguage(navigator.languages ?? [navigator.language]);
  }
}
