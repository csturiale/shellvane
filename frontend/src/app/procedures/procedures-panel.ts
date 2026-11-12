import { Component, ElementRef, Injector, afterRenderEffect, computed, effect, inject, signal, untracked, viewChild } from '@angular/core';
import { DomSanitizer } from '@angular/platform-browser';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { LucideArrowLeft, LucideLink, LucidePin, LucidePencil, LucidePlus, LucideShare2, LucideRefreshCw, LucideRotateCcw, LucideSearch, LucideTrash2, LucideVariable, LucideWorkflow } from '@lucide/angular';

import { DangerousCommands } from '../broadcast/dangerous-commands';
import { writeClipboard } from '../core/clipboard';
import { HelpLink } from '../guide/help-link';
import { Navigation } from '../layout/navigation';
import { SessionsStore } from '../sessions/sessions.store';
import { ConfirmDialog } from '../shared/confirm-dialog';
import { toTerminalInput } from '../snippets/variables';
import { CodeBlock, fill, placeholders, renderProcedure } from './markdown';
import { ProceduresStore, SavedSource } from './procedures.store';
import { SourceDialog } from './source-dialog';
import { ValuesAnswer, ValuesDialog } from './values-dialog';
import { PersonalValues } from '../core/personal-values';
import { WebPageDialog } from './web-page-dialog';
import { AutomationApi } from '../core/backend/automation-api';
import { Sharing } from '../extensions/sharing';
import { ContextMenuService } from '../shared/context-menu.service';

/**
 * Procedures beside the terminals: Markdown from a Git repository or a
 * folder, whose commands are pasted or run in the focused pane without
 * switching window. Nothing runs by itself: every command is a click.
 */
@Component({
  selector: 'app-procedures-panel',
  imports: [
    TranslocoDirective,
    HelpLink,
    ConfirmDialog,
    SourceDialog,
    ValuesDialog,
    WebPageDialog,
    LucideArrowLeft,
    LucideLink,
    LucideTrash2,
    LucidePencil,
    LucidePlus,
    LucideRefreshCw,
    LucideRotateCcw,
    LucideSearch,
    LucideVariable,
    LucidePin,
    LucideShare2,
    LucideWorkflow,
  ],
  templateUrl: './procedures-panel.html',
  styleUrl: './procedures-panel.scss',
})
export class ProceduresPanel {
  protected readonly store = inject(ProceduresStore);
  private readonly personal = inject(PersonalValues);
  private readonly sessions = inject(SessionsStore);
  private readonly automation = inject(AutomationApi);
  private readonly danger = inject(DangerousCommands);
  private readonly transloco = inject(TranslocoService);
  private readonly sanitizer = inject(DomSanitizer);
  private readonly injector = inject(Injector);
  private readonly nav = inject(Navigation);
  private readonly sharing = inject(Sharing);
  private readonly menu = inject(ContextMenuService);

  protected readonly filter = signal('');
  protected readonly editing = signal<SavedSource | 'new' | null>(null);
  protected readonly asking = signal<{ names: string[]; then: (values: Record<string, string>) => void } | null>(null);
  protected readonly confirming = signal<{ text: string; run: boolean; index: number; message: string } | null>(null);
  protected readonly message = signal<string | null>(null);
  protected readonly webOpen = signal(false);
  /** The environment of the focused terminal, for its linked procedures. */
  protected readonly focusedEnv = computed(() => this.sessions.focused()?.env ?? null);
  protected readonly envLinks = computed(() => {
    const env = this.focusedEnv();
    this.store.links(); // follows changes
    return env ? this.store.linksOf(env.id) : [];
  });
  private readonly article = viewChild<ElementRef<HTMLElement>>('article');

  protected readonly rendered = computed(() => {
    const d = this.store.doc();
    if (!d) {
      return null;
    }
    const t = (k: string) => this.transloco.translate('procedures.' + k);
    return renderProcedure(d.markdown, d.path, { paste: t('paste'), run: t('run'), copy: t('copy'), done: t('done') });
  });
  protected readonly html = computed(() => this.sanitizer.bypassSecurityTrustHtml(this.rendered()?.html ?? ''));
  /** Placeholders of the procedure's commands, in order. */
  protected readonly names = computed(() => [...new Set((this.rendered()?.blocks ?? []).filter((b) => b.shell).flatMap((b) => placeholders(b.command)))]);
  protected readonly progress = computed(() => {
    const blocks = (this.rendered()?.blocks ?? []).filter((b) => b.shell);
    return { done: blocks.filter((b) => this.store.done().has(b.index)).length, total: blocks.length };
  });
  protected readonly filtered = computed(() => {
    const q = this.filter().trim().toLowerCase();
    return q ? this.store.docs().filter((d) => d.title.toLowerCase().includes(q) || d.path.toLowerCase().includes(q)) : this.store.docs();
  });

  constructor() {
    // Repositories shared by the user's teams (none without the team extension).
    void this.sharing.procedureSources().then(
      (list) => this.store.teamSources.set(list),
      () => undefined,
    );
    void this.store.loadInbox(this.transloco.translate('procedures.inbox')).then(() => {
      if (this.store.current() && this.store.docs().length === 0 && !this.store.loading() && !this.nav.procedure()) {
        void this.store.load();
      }
    });
    // A page sent from the browser opens here.
    effect(() => {
      const path = this.nav.procedure();
      if (path) {
        untracked(() => {
          this.nav.procedure.set(null);
          void this.store.loadInbox(this.transloco.translate('procedures.inbox')).then(() => this.store.openReceived(path));
        });
      }
    });
    // Marks the steps done in the rendered text.
    afterRenderEffect(() => {
      const el = this.article()?.nativeElement;
      const done = this.store.done();
      this.html();
      el?.querySelectorAll<HTMLElement>('.proc-code').forEach((b) => b.classList.toggle('done', done.has(Number(b.dataset['i']))));
    });
  }

  protected async saveWeb(page: { title: string; markdown: string }): Promise<void> {
    this.webOpen.set(false);
    await this.store.saveInbox(page.title, '', page.markdown).catch((err) => this.message.set(String(err?.message ?? err)));
  }

  /** The teams a Git source can be shared with (none without the team extension). */
  protected shareMenu(ev: MouseEvent, src: SavedSource): void {
    const items = this.sharing.procedureShareMenu({ name: src.name, url: src.url ?? '', branch: src.branch ?? '', folder: src.folder ?? '' }, (work) => {
      void work().then(
        () => this.message.set(this.transloco.translate('procedures.sharedSource', { name: src.name })),
        (err) => this.message.set(String((err as { message?: string })?.message ?? err)),
      );
    });
    if (items.length > 0) {
      this.menu.open(ev, items);
    }
  }

  protected canShare(src: SavedSource): boolean {
    return src.kind === 'git' && !this.store.isTeam() && this.sharing.procedureShareMenu({ name: '', url: '', branch: '', folder: '' }, () => undefined).length > 0;
  }

  protected onSource(ev: Event): void {
    void this.store.select((ev.target as HTMLSelectElement).value);
  }

  protected saveSource(s: Omit<SavedSource, 'id'> & { id?: string }): void {
    this.editing.set(null);
    if (s.id) {
      this.store.updateSource(s as SavedSource);
    } else {
      this.store.addSource(s);
    }
  }

  protected removeSource(id: string): void {
    this.editing.set(null);
    this.store.removeSource(id);
  }

  /** Clicks in the procedure: the actions of a code block, links to other procedures. */
  protected onClick(ev: MouseEvent): void {
    const target = ev.target as HTMLElement;
    const link = target.closest<HTMLAnchorElement>('a.doc-link');
    if (link) {
      ev.preventDefault();
      void this.store.open(link.dataset['doc'] ?? '');
      return;
    }
    const button = target.closest<HTMLElement>('[data-act]');
    const block = this.rendered()?.blocks[Number(button?.dataset['i'])];
    if (!button || !block) {
      return;
    }
    this.act(button.dataset['act'] ?? '', block);
  }

  private act(kind: string, block: CodeBlock): void {
    this.message.set(null);
    if (kind === 'done') {
      this.store.markDone(block.index, !this.store.done().has(block.index));
      return;
    }
    const source = kind === 'copy' ? block.code : block.command;
    this.withValues(source, (text) => {
      if (kind === 'copy') {
        void writeClipboard(text).then(() => this.message.set(this.transloco.translate('procedures.copied')));
      } else {
        this.send(text, kind === 'run', block.index);
      }
    });
  }

  /** Fills the placeholders, asking those without a value first. */
  private withValues(text: string, then: (filled: string) => void): void {
    const missing = placeholders(fill(text, this.values(placeholders(text))));
    if (missing.length === 0) {
      then(fill(text, this.values(placeholders(text))));
      return;
    }
    this.asking.set({
      names: missing,
      then: () => then(fill(text, this.values(placeholders(text)))),
    });
  }

  /** The source's scope for its own personal values. */
  protected scope(): string {
    return `procedure:${this.store.current()?.id ?? ''}`;
  }

  /** Values of names: typed in this session first, then personal ones. */
  /**
   * Values of names: typed in this session first, then those of the
   * environment the focused terminal belongs to (its variables, e.g. a
   * namespace or a project), then the personal ones.
   */
  private values(names: string[]): Record<string, string> {
    const env = this.sessions.focused()?.env?.values ?? {};
    const fromEnv = Object.fromEntries(names.filter((n) => env[n]).map((n) => [n, env[n]]));
    return { ...this.personal.resolve(names, this.scope()), ...fromEnv, ...nonEmpty(this.store.values()) };
  }

  protected answer(answer: ValuesAnswer): void {
    const a = this.asking();
    this.asking.set(null);
    this.store.setValues(answer.values);
    for (const [name, value] of Object.entries(answer.values)) {
      this.personal.remember(name, value, answer.where[name] ?? 'none', this.scope());
    }
    a?.then(answer.values);
  }

  protected editValues(): void {
    this.asking.set({ names: this.names(), then: () => undefined });
  }

  private send(text: string, run: boolean, index: number): void {
    const focused = this.sessions.focused();
    if (!focused) {
      this.message.set(this.transloco.translate('procedures.noTerminal'));
      return;
    }
    const receivers = this.sessions.receivers(focused.key).length;
    if (receivers > 1 && this.danger.test(text)) {
      this.confirming.set({ text, run, index, message: this.transloco.translate('snippets.dangerous', { count: receivers, command: text }) });
      return;
    }
    this.deliver(text, run, index);
  }

  protected deliver(text: string, run: boolean, index: number): void {
    this.confirming.set(null);
    const focused = this.sessions.focused();
    const count = this.sessions.sendToFocused(toTerminalInput(text, run));
    this.store.markDone(index);
    // The audit log keeps what was sent where (Automation › Audit log).
    const doc = this.store.doc();
    if (focused && doc) {
      const where = [focused.env?.name, focused.title].filter(Boolean).join(' · ') + (count > 1 ? ` (+${count - 1})` : '');
      void this.automation.recordProcedure(run ? 'run' : 'paste', where, doc.title || doc.path, text).catch(() => undefined);
    }
    this.message.set(count > 1 ? this.transloco.translate('snippets.sentTo', { count }) : null);
  }

  /** A runbook with the procedure's commands as steps, placeholders as variables. */
  protected async toRunbook(): Promise<void> {
    const doc = this.store.doc();
    const blocks = (this.rendered()?.blocks ?? []).filter((b) => b.shell);
    if (!doc || blocks.length === 0) {
      return;
    }
    const asVariable = (cmd: string) => cmd.replace(/<([A-Za-z][\w.-]*)>/g, '{{$1}}');
    try {
      // Loaded on demand: the runbooks belong to the Automation view.
      const { RunbooksStore } = await import('../runbooks/runbooks.store');
      await this.injector.get(RunbooksStore).save({
        id: '',
        name: doc.title,
        description: this.transloco.translate('procedures.runbookFrom', { path: doc.path }),
        variables: this.names().map((name) => ({ name, default: this.values([name])[name] ?? '', choices: [] })),
        steps: blocks.map((b, i) => ({ name: `step${i + 1}`, command: asVariable(b.command), when: '', pattern: '', continueOnError: false, confirm: false, timeoutSec: 0 })),
        targets: { hostIds: [], groupIds: [], tags: [] },
        parallel: false,
        concurrency: 0,
        stopOnHostFailure: true,
      });
      this.nav.view.set('automation');
    } catch (err) {
      this.message.set(String((err as { message?: string })?.message ?? err));
    }
  }
}

function nonEmpty(values: Record<string, string>): Record<string, string> {
  return Object.fromEntries(Object.entries(values).filter(([, v]) => v !== ''));
}
