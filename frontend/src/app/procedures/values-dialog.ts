import { Component, inject, input, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective } from '@jsverse/transloco';

import { PersonalValues, Remember } from '../core/personal-values';
import { Modal } from '../shared/modal';
import { RememberSelect } from '../shared/remember-select';

/** The values typed, and where each is remembered. */
export interface ValuesAnswer {
  values: Record<string, string>;
  where: Record<string, Remember>;
}

/** Asks the values of a procedure's placeholders (<name>, {{name}}). */
@Component({
  selector: 'app-procedure-values-dialog',
  imports: [FormsModule, TranslocoDirective, Modal, RememberSelect],
  templateUrl: './values-dialog.html',
  styleUrl: './values-dialog.scss',
})
export class ValuesDialog {
  private readonly personal = inject(PersonalValues);
  readonly names = input.required<string[]>();
  readonly initial = input<Record<string, string>>({});
  /** The scope of the source's own personal values. */
  readonly scope = input('');
  readonly done = output<ValuesAnswer>();
  readonly closed = output<void>();

  protected values: Record<string, string> = {};
  protected where: Record<string, Remember> = {};

  ngOnInit(): void {
    for (const n of this.names()) {
      this.values[n] = this.initial()[n] || (this.personal.get(n, this.scope() || undefined) ?? '');
      this.where[n] = this.personal.source(n, this.scope() || undefined);
    }
  }

  protected submit(): void {
    this.done.emit({ values: { ...this.values }, where: { ...this.where } });
  }
}
