import { Component, computed, input, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective } from '@jsverse/transloco';

import { Snippet } from '../core/backend/snippet-api';
import { Modal } from '../shared/modal';
import { userVariables } from './variables';

/** Asks the values of a snippet's variables before it is sent. */
@Component({
  selector: 'app-run-dialog',
  imports: [FormsModule, TranslocoDirective, Modal],
  templateUrl: './run-dialog.html',
  styleUrl: '../shared/form.scss',
})
export class RunDialog {
  readonly snippet = input.required<Snippet>();
  readonly run = output<Record<string, string>>();
  readonly closed = output<void>();

  protected readonly variables = computed(() => userVariables(this.snippet().body));
  protected values: Record<string, string> = {};

  ngOnInit(): void {
    for (const v of this.variables()) {
      this.values[v.name] = v.choices[0] ?? '';
    }
  }
}
