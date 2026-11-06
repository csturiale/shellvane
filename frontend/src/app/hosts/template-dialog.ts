import { Component, computed, inject, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';

import { TranslatableError, toTranslatableError } from '../core/backend/errors';
import { readClipboard } from '../core/clipboard';
import { HelpLink } from '../guide/help-link';
import { Modal } from '../shared/modal';
import { HostsStore } from './hosts.store';
import { groupPaths } from './tree';

/** Pastes a group template copied from another Shellvane and creates its groups. */
@Component({
  selector: 'app-template-dialog',
  imports: [FormsModule, HelpLink, Modal, TranslocoDirective],
  templateUrl: './template-dialog.html',
  styleUrls: ['../shared/form.scss', './template-dialog.scss'],
})
export class TemplateDialog {
  /** Emits a message describing the import, or null when cancelled. */
  readonly closed = output<string | null>();

  private readonly store = inject(HostsStore);
  private readonly transloco = inject(TranslocoService);
  protected readonly groups = computed(() => groupPaths(this.store.groups()).sort((a, b) => a.path.localeCompare(b.path)));
  protected text = '';
  protected parentId = '';
  protected readonly busy = signal(false);
  protected readonly error = signal<TranslatableError | null>(null);

  protected async paste(): Promise<void> {
    this.text = (await readClipboard()) || this.text;
  }

  protected async submit(): Promise<void> {
    this.busy.set(true);
    this.error.set(null);
    try {
      const r = await this.store.importTemplate(this.parentId, this.text);
      let msg = this.transloco.translate('hosts.template.imported', { created: r.created, reused: r.reused, hosts: r.hostsCreated, skipped: r.hostsSkipped });
      if (r.commandsLeftOut?.length) {
        msg += ' ' + this.transloco.translate('hosts.template.commandsLeftOut', { commands: r.commandsLeftOut.join('; ') });
      }
      this.closed.emit(msg);
    } catch (e) {
      this.error.set(toTranslatableError(e));
    } finally {
      this.busy.set(false);
    }
  }
}
