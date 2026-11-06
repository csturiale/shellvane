import { Component, computed, inject, signal } from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';

import { toTranslatableError } from '../core/backend/errors';
import { Theme } from '../core/backend/theme-api';
import { writeClipboard } from '../core/clipboard';
import { HelpLink } from '../guide/help-link';
import { ConfirmDialog } from '../shared/confirm-dialog';
import { ThemeEditor } from '../themes/theme-editor';
import { ThemesStore, toTheme } from '../themes/themes.store';

import { ThemeService } from '../core/theme/theme.service';
import { AUTO_SCHEME, allSchemes, findScheme } from './schemes';
import { CursorStyle, FONT_SIZES, SCROLLBACK_SIZES, TerminalPrefs } from './terminal-prefs';
import { CommandHistory } from './suggest';
import { GalleryDialog } from '../gallery/gallery-dialog';

/** Terminal section of the settings page, with a live preview. */
@Component({
  selector: 'app-terminal-settings',
  imports: [GalleryDialog, TranslocoDirective, HelpLink, ConfirmDialog, ThemeEditor],
  templateUrl: './terminal-settings.html',
  styleUrl: './terminal-settings.scss',
})
export class TerminalSettings {
  protected readonly prefs = inject(TerminalPrefs);
  protected readonly history = inject(CommandHistory);
  private readonly theme = inject(ThemeService);
  protected readonly schemes = allSchemes;
  protected readonly themes = inject(ThemesStore);
  private readonly transloco = inject(TranslocoService);
  protected readonly galleryOpen = signal(false);
  protected readonly editing = signal<Theme | null>(null);
  protected readonly deleting = signal<Theme | null>(null);
  protected readonly message = signal<string | null>(null);
  /** The selected scheme when it is a custom one. */
  protected readonly current = computed(() => this.themes.themes().find((t) => t.id === this.prefs.scheme()) ?? null);
  protected readonly sizes = FONT_SIZES;
  protected readonly auto = AUTO_SCHEME;
  protected readonly preview = computed(() => findScheme(this.prefs.scheme(), this.theme.isDark()));
  protected readonly ansi = ['red', 'green', 'yellow', 'blue', 'magenta', 'cyan'] as const;
  protected readonly cursors: CursorStyle[] = ['block', 'bar', 'underline'];
  protected readonly scrollbacks = SCROLLBACK_SIZES;
  protected readonly logFormats = ['text', 'timestamped', 'raw'];
  protected readonly toggles = ['copyOnSelect', 'confirmPaste', 'confirmClose', 'bellNotify', 'altIsMeta', 'backspaceCtrlH', 'screenReader'] as const;

  protected onScheme(ev: Event): void {
    this.prefs.setScheme((ev.target as HTMLSelectElement).value);
  }

  protected newTheme(): void {
    this.editing.set(toTheme(findScheme('auto', this.theme.isDark()), this.transloco.translate('themes.untitled')));
  }

  /** Starts a new scheme from the selected one. */
  protected duplicate(): void {
    const s = this.preview();
    this.editing.set(toTheme(s, this.transloco.translate('themes.copyOf', { name: s.name })));
  }

  protected onSaved(t: Theme): void {
    this.editing.set(null);
    this.prefs.setScheme(t.id);
  }

  protected async importFiles(ev: Event): Promise<void> {
    const input = ev.target as HTMLInputElement;
    const names: string[] = [];
    try {
      for (const f of Array.from(input.files ?? [])) {
        const imported = await this.themes.import(f.name, await f.text());
        names.push(...imported.map((t) => t.name));
        if (imported.length > 0) {
          this.prefs.setScheme(imported[imported.length - 1].id);
        }
      }
      this.message.set(this.transloco.translate('themes.imported', { names: names.join(', ') }));
    } catch (err) {
      this.fail(err);
    } finally {
      input.value = '';
    }
  }

  protected async exportTheme(t: Theme): Promise<void> {
    try {
      await writeClipboard(await this.themes.export(t));
      this.message.set(this.transloco.translate('themes.exported', { name: t.name }));
    } catch (err) {
      this.fail(err);
    }
  }

  protected async confirmDelete(t: Theme): Promise<void> {
    this.deleting.set(null);
    await this.themes.delete(t.id).catch((err) => this.fail(err));
  }

  private fail(err: unknown): void {
    const e = toTranslatableError(err);
    this.message.set(this.transloco.translate(e.key, e.params));
  }

  protected onSize(ev: Event): void {
    this.prefs.setFontSize(Number((ev.target as HTMLSelectElement).value));
  }
}
