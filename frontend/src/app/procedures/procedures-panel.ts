import { Component, ElementRef, Injector, afterRenderEffect, computed, effect, inject, signal, untracked, viewChild } from '@angular/core';
import { DomSanitizer } from '@angular/platform-browser';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { LucideArrowLeft, LucideLink, LucidePencil, LucidePlus, LucideRefreshCw, LucideRotateCcw, LucideSearch, LucideTrash2, LucideVariable, LucideWorkflow } from '@lucide/angular';

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
import { ValuesDialog } from './values-dialog';
import { WebPageDialog } from './web-page-dialog';

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
    LucideWorkflow,
  ],
  templateUrl: './procedures-panel.html',
  styleUrl: './procedures-panel.scss',
})
export class ProceduresPanel {
  protected readonly store = inject(ProceduresStore);
  private readonly sessions = inject(SessionsStore);
  private readonly danger = inject(DangerousCommands);
  private readonly transloco = inject(TranslocoService);
  private readonly sanitizer = inject(DomSanitizer);
  private readonly injector = inject(Injector);
  private readonly nav = inject(Navigation);

  protected readonly filter = signal('');
  protected readonly editing = signal<SavedSource | 'new' | null>(null);
  protected readonly asking = signal<{ names: string[]; then: (values: Record<string, string>) => void } | null>(null);
  protected readonly confirming = signal<{ text: string; run: boolean; index: number; message: string } | null>(null);
  protected readonly message = signal<string | null>(null);
  protected readonly webOpen = signal(false);
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
    const missing = placeholders(fill(text, this.store.values()));
    if (missing.length === 0) {
      then(fill(text, this.store.values()));
      return;
    }
    this.asking.set({
      names: missing,
      then: (values) => {
        this.store.setValues(values);
        then(fill(text, this.store.values()));
      },
    });
  }

  protected answer(values: Record<string, string>): void {
    const a = this.asking();
    this.asking.set(null);
    a?.then(values);
  }

  protected editValues(): void {
    this.asking.set({ names: this.names(), then: (values) => this.store.setValues(values) });
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
    const count = this.sessions.sendToFocused(toTerminalInput(text, run));
    this.store.markDone(index);
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
        variables: this.names().map((name) => ({ name, default: this.store.values()[name] ?? '', choices: [] })),
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
