import { Component, ElementRef, computed, inject, input, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { LucideShipWheel } from '@lucide/angular';

import { toTranslatableError } from '../core/backend/errors';
import { SessionApi } from '../core/backend/session-api';
import { SessionPrefs } from './session-prefs';
import { Pane, SessionsStore } from './sessions.store';
import { HelpLink } from '../guide/help-link';

/** The pane's kubectl context: a badge that opens a picker. */
@Component({
  selector: 'app-kube-picker',
  imports: [HelpLink, FormsModule, TranslocoDirective, LucideShipWheel],
  templateUrl: './kube-picker.html',
  styleUrl: './kube-picker.scss',
  host: { '(document:pointerdown)': 'onOutside($event)', '(keydown.escape)': 'open.set(false)', '[class.unavailable]': 'unavailable()' },
})
export class KubePicker {
  readonly pane = input.required<Pane>();

  private readonly api = inject(SessionApi);
  private readonly store = inject(SessionsStore);
  private readonly transloco = inject(TranslocoService);
  private readonly el = inject(ElementRef<HTMLElement>);
  protected readonly prefs = inject(SessionPrefs);

  protected readonly open = signal(false);
  /** The menu is fixed: the pane header clips what overflows it. */
  protected readonly pos = signal({ top: 0, left: 0 });
  protected readonly loading = signal(false);
  protected readonly contexts = signal<string[]>([]);
  protected readonly error = signal<string | null>(null);
  protected context = '';
  protected namespace = '';

  /** Sessions where kubectl is not installed: their badge goes away once
   * the picker has said so (it would only fail again). */
  private static readonly missing = signal(new Set<string>());
  protected readonly unavailable = computed(
    () => !this.open() && !this.pane().kube?.context && KubePicker.missing().has(this.pane().sessionId),
  );

  protected readonly current = computed(() => this.pane().kube);
  protected readonly prod = computed(() => this.prefs.isProd(this.current()?.context ?? ''));

  protected async toggle(ev: MouseEvent): Promise<void> {
    const box = (ev.currentTarget as HTMLElement).getBoundingClientRect();
    const width = Math.min(320, globalThis.innerWidth - 16);
    this.pos.set({ top: box.bottom + 6, left: Math.max(8, Math.min(box.left, globalThis.innerWidth - width - 8)) });
    this.open.update((o) => !o);
    if (!this.open()) {
      return;
    }
    this.context = this.current()?.context ?? '';
    this.namespace = this.current()?.namespace ?? '';
    this.error.set(null);
    this.loading.set(true);
    try {
      this.contexts.set(await this.api.kubeContexts(this.pane().sessionId));
    } catch (err) {
      this.contexts.set([]);
      this.fail(err);
      if (toTranslatableError(err).key === 'errors.kube.notInstalled') {
        KubePicker.missing.update((s) => new Set(s).add(this.pane().sessionId));
      }
    } finally {
      this.loading.set(false);
    }
  }

  protected async apply(): Promise<void> {
    try {
      await this.store.setKube(this.pane().key, { context: this.context, namespace: this.context ? this.namespace.trim() : '' });
      this.open.set(false);
    } catch (err) {
      this.fail(err);
    }
  }

  protected onOutside(ev: PointerEvent): void {
    if (this.open() && !this.el.nativeElement.contains(ev.target as Node)) {
      this.open.set(false);
    }
  }

  private fail(err: unknown): void {
    const e = toTranslatableError(err);
    this.error.set(this.transloco.translate(e.key, e.params));
  }
}
