import { Component, computed, inject, input, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';

import { toTranslatableError } from '../core/backend/errors';
import { Theme } from '../core/backend/theme-api';
import { Modal } from '../shared/modal';
import { COLOR_KEYS, ThemesStore } from './themes.store';

/** Editor of a custom scheme, with a live preview. */
@Component({
  selector: 'app-theme-editor',
  imports: [FormsModule, TranslocoDirective, Modal],
  templateUrl: './theme-editor.html',
  styleUrls: ['../shared/form.scss', './theme-editor.scss'],
})
export class ThemeEditor {
  /** The theme to edit; an empty ID creates one. */
  readonly theme = input.required<Theme>();
  readonly closed = output<void>();
  readonly saved = output<Theme>();

  private readonly store = inject(ThemesStore);
  private readonly transloco = inject(TranslocoService);

  protected readonly keys = COLOR_KEYS;
  protected readonly ansi = COLOR_KEYS.slice(5);
  protected name = '';
  protected readonly colors = signal<Record<string, string>>({});
  protected readonly error = signal<string | null>(null);
  protected readonly saving = signal(false);

  protected readonly c = computed(() => {
    const c = this.colors();
    return (k: string) => c[k] || (k === 'cursor' ? c['foreground'] : k === 'selectionBackground' ? c['brightBlack'] : '') || '';
  });

  ngOnInit(): void {
    this.name = this.theme().name;
    this.colors.set({ ...this.theme().colors });
  }

  protected setColor(key: string, value: string): void {
    const v = value.trim().toLowerCase();
    this.colors.update((c) => ({ ...c, [key]: v }));
  }

  protected valid(v: string | undefined): boolean {
    return !!v && /^#[0-9a-f]{6}$/i.test(v);
  }

  protected async save(): Promise<void> {
    this.saving.set(true);
    this.error.set(null);
    try {
      const colors = Object.fromEntries(Object.entries(this.colors()).filter(([, v]) => this.valid(v)));
      const t = await this.store.save({ ...this.theme(), name: this.name, colors });
      this.saved.emit(t);
    } catch (err) {
      const e = toTranslatableError(err);
      this.error.set(this.transloco.translate(e.key, e.params));
    } finally {
      this.saving.set(false);
    }
  }
}
