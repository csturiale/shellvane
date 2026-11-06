import { Component, computed, inject, input, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective } from '@jsverse/transloco';

import { toTranslatableError, TranslatableError } from '../core/backend/errors';
import { FileEntry, FilesApi } from '../core/backend/files-api';
import { Modal } from '../shared/modal';
import { SPECIAL, Special, WHAT, WHO, What, Who, bit, parseOctal, specialBit, toOctal, toSymbolic } from './permissions';

@Component({
  selector: 'app-permissions-dialog',
  imports: [FormsModule, TranslocoDirective, Modal],
  templateUrl: './permissions-dialog.html',
  styleUrls: ['../shared/form.scss', './permissions-dialog.scss'],
})
export class PermissionsDialog {
  readonly fs = input.required<string>();
  readonly entry = input.required<FileEntry>();
  readonly closed = output<void>();
  /** Emitted after the permissions changed. */
  readonly changed = output<void>();

  private readonly api = inject(FilesApi);
  protected readonly who = WHO;
  protected readonly what = WHAT;
  protected readonly special = SPECIAL;

  protected readonly perm = signal(0);
  protected readonly octal = computed(() => toOctal(this.perm()));
  protected readonly symbolic = computed(() => toSymbolic(this.perm()));
  protected recursive = false;
  /** Numeric owner and group; empty leaves them unchanged. */
  protected uid: number | string = '';
  protected gid: number | string = '';
  protected readonly error = signal<TranslatableError | null>(null);
  protected readonly saving = signal(false);

  ngOnInit(): void {
    this.perm.set(this.entry().perm);
  }

  protected has(who: Who, what: What): boolean {
    return (this.perm() & bit(who, what)) !== 0;
  }

  protected hasSpecial(s: Special): boolean {
    return (this.perm() & specialBit(s)) !== 0;
  }

  protected toggle(mask: number): void {
    this.perm.update((p) => p ^ mask);
  }

  protected bit = bit;
  protected specialBit = specialBit;

  protected onOctal(ev: Event): void {
    const v = parseOctal((ev.target as HTMLInputElement).value);
    if (v !== null) {
      this.perm.set(v);
    }
  }

  protected async save(): Promise<void> {
    this.saving.set(true);
    this.error.set(null);
    try {
      await this.api.chmod(this.fs(), this.entry().path, this.perm(), this.entry().isDir && this.recursive);
      const e = this.entry();
      const uid = this.uid === '' ? e.uid : Number(this.uid);
      const gid = this.gid === '' ? e.gid : Number(this.gid);
      if ((uid !== e.uid || gid !== e.gid) && uid >= 0 && gid >= 0) {
        await this.api.chown(this.fs(), e.path, uid, gid);
      }
      this.changed.emit();
      this.closed.emit();
    } catch (err) {
      this.error.set(toTranslatableError(err));
    } finally {
      this.saving.set(false);
    }
  }
}
