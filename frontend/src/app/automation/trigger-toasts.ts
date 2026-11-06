import { Component, DestroyRef, inject, signal } from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { LucideX } from '@lucide/angular';

import { AutomationApi, TriggerEvent } from '../core/backend/automation-api';
import { toTranslatableError } from '../core/backend/errors';
import { panesOf } from '../sessions/layout';
import { SessionsStore } from '../sessions/sessions.store';
import { Navigation } from '../layout/navigation';
import { FULL_VIEWS } from '../layout/views';

interface Toast {
  id: number;
  kind: string;
  title: string;
  text: string;
  sessionId: string;
}

const TOAST_MS = 8000;
const MAX_TOASTS = 4;

/** Shows what output rules and startup scripts did, in-app and natively. */
@Component({
  selector: 'app-trigger-toasts',
  imports: [TranslocoDirective, LucideX],
  templateUrl: './trigger-toasts.html',
  styleUrl: './trigger-toasts.scss',
})
export class TriggerToasts {
  private readonly sessions = inject(SessionsStore);
  private readonly nav = inject(Navigation);
  private readonly transloco = inject(TranslocoService);
  protected readonly toasts = signal<Toast[]>([]);
  private next = 1;

  constructor() {
    try {
      const off = inject(AutomationApi).onTrigger((e) => this.show(e));
      inject(DestroyRef).onDestroy(off);
    } catch {
      // No event bus outside the desktop app.
    }
  }

  protected show(e: TriggerEvent): void {
    const t = (k: string, p?: object) => this.transloco.translate(k, p);
    const session = this.sessions.titleOf(e.sessionId) || '?';
    let text = e.line;
    if (e.error) {
      const err = toTranslatableError({ cause: e.error });
      text = t(err.key, err.params);
    }
    const toast: Toast = {
      id: this.next++,
      kind: e.kind,
      title: t('automation.events.' + e.kind, { rule: e.rule, session }),
      text,
      sessionId: e.sessionId,
    };
    this.toasts.update((list) => [...list, toast].slice(-MAX_TOASTS));
    setTimeout(() => this.dismiss(toast.id), TOAST_MS);
    if (e.kind === 'notify') {
      this.native(toast);
      if (e.sound) {
        beep();
      }
    }
  }

  protected dismiss(id: number): void {
    this.toasts.update((list) => list.filter((x) => x.id !== id));
  }

  /** Brings the pane of the event to the front. */
  protected reveal(toast: Toast): void {
    const pane = this.sessions.panes().find((p) => p.sessionId === toast.sessionId);
    const tab = pane && this.sessions.tabs().find((t) => panesOf(t.layout).includes(pane.key));
    if (pane && tab) {
      if (FULL_VIEWS.includes(this.nav.view())) {
        this.nav.view.set('hosts');
      }
      this.sessions.activeKey.set(tab.key);
      this.sessions.focusPane(tab.key, pane.key);
    }
    this.dismiss(toast.id);
  }

  /** A system notification when the app is not in front. */
  private native(toast: Toast): void {
    if (!('Notification' in globalThis) || Notification.permission !== 'granted' || document.hasFocus()) {
      return;
    }
    try {
      const n = new Notification(toast.title, { body: toast.text });
      n.onclick = () => {
        globalThis.focus();
        this.reveal(toast);
      };
    } catch {
      // Not supported by this webview.
    }
  }
}

/** A short two-tone alert, without an audio file. */
function beep(): void {
  try {
    const ctx = new AudioContext();
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();
    osc.connect(gain);
    gain.connect(ctx.destination);
    gain.gain.value = 0.08;
    osc.frequency.setValueAtTime(880, ctx.currentTime);
    osc.frequency.setValueAtTime(660, ctx.currentTime + 0.12);
    osc.start();
    osc.stop(ctx.currentTime + 0.25);
    osc.onended = () => void ctx.close();
  } catch {
    // No audio.
  }
}
