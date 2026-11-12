import { Component, input, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective } from '@jsverse/transloco';

import { Modal } from '../shared/modal';

/** Asks the values of a procedure's placeholders (<name>, {{name}}). */
@Component({
  selector: 'app-procedure-values-dialog',
  imports: [FormsModule, TranslocoDirective, Modal],
  templateUrl: './values-dialog.html',
  styleUrl: '../shared/form.scss',
})
export class ValuesDialog {
  readonly names = input.required<string[]>();
  readonly initial = input<Record<string, string>>({});
  readonly done = output<Record<string, string>>();
  readonly closed = output<void>();

  protected values: Record<string, string> = {};

  ngOnInit(): void {
    for (const n of this.names()) {
      this.values[n] = this.initial()[n] ?? '';
    }
  }
}
