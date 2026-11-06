import { Component, ElementRef, computed, effect, inject, signal, untracked, viewChild } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { DomSanitizer, SafeHtml } from '@angular/platform-browser';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { LucideCircleCheck, LucideCircle, LucideSearch } from '@lucide/angular';

import { readPreference } from '../core/preferences';
import { HostsStore } from '../hosts/hosts.store';
import { KeysStore } from '../keys/keys.store';
import { Navigation } from '../layout/navigation';
import { SessionsStore } from '../sessions/sessions.store';
import { TunnelsStore } from '../tunnels/tunnels.store';
import { VaultStore } from '../vault/vault.store';
import { ALL_GUIDE_TOPICS, GuideSection, excerpt, parseAction, search } from './guide';
import { render } from './guide-render';
import { GuideStore } from './guide.store';

interface Step {
  id: string;
  done: boolean;
  action: string;
}

/** The in-app guide: topics, search, first steps and "try it" links. */
@Component({
  selector: 'app-guide-view',
  imports: [TranslocoDirective, LucideSearch, LucideCircleCheck, LucideCircle],
  templateUrl: './guide-view.html',
  styleUrl: './guide-view.scss',
})
export class GuideView {
  protected readonly nav = inject(Navigation);
  private readonly store = inject(GuideStore);
  private readonly sanitizer = inject(DomSanitizer);
  private readonly transloco = inject(TranslocoService);
  private readonly sessions = inject(SessionsStore);
  private readonly hosts = inject(HostsStore);
  private readonly keys = inject(KeysStore);
  private readonly tunnels = inject(TunnelsStore);
  private readonly vault = inject(VaultStore);
  private readonly content = viewChild<ElementRef<HTMLElement>>('content');

  protected readonly topics = ALL_GUIDE_TOPICS;
  protected readonly html = signal<SafeHtml>('');
  protected readonly query = signal('');
  protected readonly results = computed(() => search(this.store.index(), this.query()));
  protected readonly excerpt = excerpt;

  /** First steps, done or not, from what exists in the app. */
  protected readonly steps = computed<Step[]>(() => [
    { id: 'vault', done: this.vault.state() === 'locked' || this.vault.state() === 'unlocked', action: 'view/settings' },
    { id: 'hosts', done: this.hosts.hosts().length > 0, action: 'action/import' },
    {
      id: 'connect',
      done: readPreference('guide.connected') === 'true' || this.sessions.panes().some((p) => p.state === 'connected'),
      action: 'action/newHost',
    },
    { id: 'key', done: this.keys.keys().length > 0, action: 'view/keys' },
    { id: 'tunnel', done: this.tunnels.tunnels().length > 0, action: 'view/tunnels' },
  ]);
  protected readonly doneCount = computed(() => this.steps().filter((s) => s.done).length);

  constructor() {
    void this.hosts.load().catch(() => undefined);
    void this.keys.load().catch(() => undefined);
    // Topic or language changed: render it, then scroll to the heading.
    effect(() => {
      const { topic, anchor } = this.nav.guideTopic();
      untracked(() => void this.show(topic, anchor));
    });
    this.transloco.langChanges$.pipe(takeUntilDestroyed()).subscribe(() => {
      const { topic, anchor } = this.nav.guideTopic();
      void this.show(topic, anchor);
      if (this.query()) {
        void this.store.loadIndex();
      }
    });
  }

  protected onSearch(ev: Event): void {
    this.query.set((ev.target as HTMLInputElement).value);
    void this.store.loadIndex();
  }

  protected open(s: Pick<GuideSection, 'topic' | 'anchor'>): void {
    this.query.set('');
    this.nav.guideTopic.set({ topic: s.topic, anchor: s.anchor === 'top' ? '' : s.anchor });
  }

  /** Links in the text: "Try it" actions and links to other topics. */
  protected onClick(ev: MouseEvent): void {
    const link = (ev.target as HTMLElement).closest('a.action') as HTMLAnchorElement | null;
    if (!link) {
      return;
    }
    ev.preventDefault();
    this.run(link.dataset['action'] ?? '');
  }

  protected run(action: string): void {
    const { kind, target, anchor } = parseAction(action);
    switch (kind) {
      case 'topic':
        this.open({ topic: target, anchor });
        break;
      case 'view':
        this.nav.view.set(target as never);
        break;
      case 'action':
        if (target === 'newLocal') {
          this.nav.view.set('hosts');
          void this.sessions.openLocal();
        } else {
          this.nav.ask('hosts', target);
        }
        break;
    }
  }

  private async show(topic: string, anchor: string): Promise<void> {
    const md = await this.store.topic(topic);
    // The guide is bundled with the app (not user or remote content), and
    // its links carry data attributes Angular's sanitizer would remove.
    this.html.set(this.sanitizer.bypassSecurityTrustHtml(render(md)));
    setTimeout(() => {
      const el = this.content()?.nativeElement;
      const target = anchor ? el?.querySelector(`#${CSS.escape(anchor)}`) : null;
      if (target) {
        target.scrollIntoView({ block: 'start' });
      } else {
        el?.scrollTo({ top: 0 });
      }
    });
  }
}
