import { NgComponentOutlet } from '@angular/common';
import { Sharing } from '../extensions/sharing';
import { Component, computed, effect, inject, signal, untracked } from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { LucideStore, LucideDownload, LucideListChecks, LucidePencil, LucidePlay, LucidePlus, LucideSearch, LucideSquareCode, LucideTrash2, LucideUpload, LucideUserRound } from '@lucide/angular';

import { toTranslatableError } from '../core/backend/errors';
import { Snippet } from '../core/backend/snippet-api';
import { SessionsStore } from '../sessions/sessions.store';
import { ConfirmDialog } from '../shared/confirm-dialog';
import { SnippetDialog } from './snippet-dialog';
import { RunDialog } from './run-dialog';
import { SnippetsStore } from './snippets.store';
import { render, snippetScope, systemValues, toTerminalInput, userVariables } from './variables';
import { RunAnswer } from './run-dialog';
import { PersonalValues } from '../core/personal-values';
import { HelpLink } from '../guide/help-link';
import { Navigation } from '../layout/navigation';
import { RunbooksStore, newRunbook } from '../runbooks/runbooks.store';
import { DangerousCommands } from '../broadcast/dangerous-commands';
import { GalleryDialog } from '../gallery/gallery-dialog';
import { UsageStore } from '../core/usage/usage.store';

type Pending = { snippet: Snippet; text: string; message: string };

@Component({
  selector: 'app-snippets-panel',
  imports: [NgComponentOutlet, GalleryDialog, HelpLink, TranslocoDirective, SnippetDialog, RunDialog, ConfirmDialog, LucidePlus, LucideStore, LucidePlay, LucideListChecks, LucidePencil, LucideTrash2, LucideSearch, LucideSquareCode, LucideDownload, LucideUpload, LucideUserRound],
  templateUrl: './snippets-panel.html',
  styleUrl: './snippets-panel.scss',
})
export class SnippetsPanel {
  private readonly usage = inject(UsageStore);
  protected readonly sharing = inject(Sharing);
  protected readonly store = inject(SnippetsStore);
  protected readonly sessions = inject(SessionsStore);
  private readonly danger = inject(DangerousCommands);
  private readonly transloco = inject(TranslocoService);
  private readonly runbooks = inject(RunbooksStore);
  private readonly nav = inject(Navigation);
  private readonly personal = inject(PersonalValues);

  /** Opens a one-step runbook with the snippet, to run it on several hosts. */
  protected runOnMany(s: Snippet): void {
    const vars = userVariables(s.body);
    const plain = Object.fromEntries(vars.map((v) => [v.name, `{{${v.name}}}`]));
    const rb = newRunbook();
    rb.name = s.name;
    rb.variables = vars.map((v) => ({ name: v.name, default: v.choices[0] ?? '', choices: v.choices }));
    rb.steps = [{ ...rb.steps[0], name: s.name, command: render(s.body, plain) }];
    this.runbooks.draft.set(rb);
    this.nav.view.set('automation');
  }

  protected readonly galleryOpen = signal(false);
  protected readonly filter = signal('');
  protected readonly groups = computed(() => {
    const q = this.filter().trim().toLowerCase();
    const map = new Map<string, Snippet[]>();
    for (const s of this.store.snippets()) {
      if (q && !s.name.toLowerCase().includes(q) && !s.body.toLowerCase().includes(q) && !s.folder.toLowerCase().includes(q)) {
        continue;
      }
      map.set(s.folder, [...(map.get(s.folder) ?? []), s]);
    }
    return [...map.entries()].map(([folder, snippets]) => ({ folder, snippets }));
  });

  protected readonly editing = signal<{ snippet: Snippet | null } | null>(null);
  /** A snippet whose values are asked before a run, or (mine) only remembered. */
  protected readonly asking = signal<{ snippet: Snippet; mine: boolean } | null>(null);
  protected readonly confirming = signal<Pending | null>(null);
  protected readonly deleting = signal<Snippet | null>(null);
  protected readonly message = signal<string | null>(null);

  constructor() {
    void this.store.load().catch((err) => this.showError(err));
    // A snippet chosen in the command palette that needs its variables or
    // a confirmation: run it from here, once the list has it.
    effect(() => {
      const id = this.store.pendingRun();
      const snippet = id ? this.store.snippets().find((s) => s.id === id) : undefined;
      if (snippet) {
        untracked(() => {
          this.store.pendingRun.set(null);
          this.run(snippet);
        });
      }
    });
  }

  protected onFilter(ev: Event): void {
    this.filter.set((ev.target as HTMLInputElement).value);
  }

  /** Runs a snippet in the focused pane (and its broadcast group). */
  protected run(snippet: Snippet): void {
    this.usage.count('snippet.run');
    if (!this.sessions.focused()) {
      this.message.set(this.transloco.translate('snippets.noTerminal'));
      return;
    }
    // Personal values already remembered are not asked again.
    const names = userVariables(snippet.body).map((v) => v.name);
    const known = this.personal.resolve(names, snippetScope(snippet.id));
    if (names.some((n) => known[n] === undefined)) {
      this.asking.set({ snippet, mine: false });
      return;
    }
    this.send(snippet, known);
  }

  /** This person's remembered values of the snippet's variables. */
  protected editMine(snippet: Snippet): void {
    this.asking.set({ snippet, mine: true });
  }

  protected hasVariables(snippet: Snippet): boolean {
    return userVariables(snippet.body).length > 0;
  }

  protected answer(a: { snippet: Snippet; mine: boolean }, answer: RunAnswer): void {
    this.asking.set(null);
    for (const [name, value] of Object.entries(answer.values)) {
      this.personal.remember(name, value, answer.where[name] ?? 'none', snippetScope(a.snippet.id));
    }
    if (!a.mine) {
      this.send(a.snippet, answer.values);
    }
  }

  protected send(snippet: Snippet, values: Record<string, string>): void {
    this.asking.set(null);
    const focused = this.sessions.focused();
    const text = render(snippet.body, { ...systemValues(focused?.target ?? null), ...values });
    const receivers = focused ? this.sessions.receivers(focused.key).length : 0;
    if (receivers > 1 && this.danger.test(text)) {
      this.confirming.set({
        snippet,
        text,
        message: this.transloco.translate('snippets.dangerous', { count: receivers, command: text }),
      });
      return;
    }
    this.deliver(snippet, text);
  }

  protected deliver(snippet: Snippet, text: string): void {
    this.confirming.set(null);
    const count = this.sessions.sendToFocused(toTerminalInput(text, snippet.run));
    this.message.set(count > 1 ? this.transloco.translate('snippets.sentTo', { count }) : null);
  }

  protected async confirmDelete(): Promise<void> {
    const s = this.deleting();
    this.deleting.set(null);
    if (s) {
      await this.store.delete(s.id).catch((err) => this.showError(err));
      this.personal.clearScope(snippetScope(s.id));
    }
  }

  /** Imports Shellvane snippets files, shell scripts or lists of commands. */
  protected async importFiles(ev: Event): Promise<void> {
    const input = ev.target as HTMLInputElement;
    let added = 0;
    let skipped = 0;
    try {
      for (const f of Array.from(input.files ?? [])) {
        const r = await this.store.import(f.name, await f.text());
        added += r.added;
        skipped += r.skipped;
      }
      this.message.set(this.transloco.translate('snippets.imported', { added, skipped }));
    } catch (err) {
      this.showError(err);
    } finally {
      input.value = '';
    }
  }

  /** Saves the shown snippets (all, or those matching the filter) as a file. */
  protected async exportFile(): Promise<void> {
    try {
      const ids = this.filter().trim() ? this.groups().flatMap((g) => g.snippets.map((s) => s.id)) : [];
      const text = await this.store.export(ids);
      const url = URL.createObjectURL(new Blob([text], { type: 'application/json' }));
      const a = document.createElement('a');
      a.href = url;
      a.download = 'shellvane-snippets.json';
      a.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      this.showError(err);
    }
  }

  private showError(err: unknown): void {
    const e = toTranslatableError(err);
    this.message.set(this.transloco.translate(e.key, e.params));
  }
}
