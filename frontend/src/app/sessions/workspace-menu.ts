import { NgComponentOutlet } from '@angular/common';
import { Sharing } from '../extensions/sharing';
import { Component, ElementRef, inject, input, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { LucideLayoutDashboard, LucideTrash2 } from '@lucide/angular';

import { toTranslatableError } from '../core/backend/errors';
import { Workspace } from '../core/backend/workspace-api';
import { SessionsStore } from './sessions.store';
import { WorkspacesStore } from './workspaces.store';
import { HelpLink } from '../guide/help-link';

/** Saves the open tabs as a named workspace and reopens saved ones. */
@Component({
  selector: 'app-workspace-menu',
  imports: [NgComponentOutlet, HelpLink, FormsModule, TranslocoDirective, LucideLayoutDashboard, LucideTrash2],
  templateUrl: './workspace-menu.html',
  styleUrl: './workspace-menu.scss',
  host: { '(document:pointerdown)': 'onOutside($event)', '(keydown.escape)': 'open.set(false)' },
})
export class WorkspaceMenu {
  protected readonly sharing = inject(Sharing);
  protected readonly store = inject(WorkspacesStore);
  protected readonly sessions = inject(SessionsStore);
  private readonly transloco = inject(TranslocoService);
  private readonly el = inject(ElementRef<HTMLElement>);

  /** Shows the name next to the icon (welcome screen). */
  readonly labelled = input(false);
  protected readonly open = signal(false);
  protected readonly error = signal<string | null>(null);
  protected name = '';

  protected async toggle(): Promise<void> {
    this.open.update((o) => !o);
    if (this.open()) {
      this.error.set(null);
      await this.store.load().catch((err) => this.fail(err));
    }
  }

  protected async save(): Promise<void> {
    try {
      await this.store.save(this.name);
      this.name = '';
      this.error.set(null);
    } catch (err) {
      this.fail(err);
    }
  }

  protected async openWorkspace(w: Workspace): Promise<void> {
    this.open.set(false);
    if (!(await this.store.open(w))) {
      this.error.set(this.transloco.translate('errors.workspaces.invalid'));
    }
  }

  protected async remove(w: Workspace): Promise<void> {
    await this.store.delete(w.id).catch((err) => this.fail(err));
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
