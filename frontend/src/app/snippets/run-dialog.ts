import { Component, computed, inject, input, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective } from '@jsverse/transloco';

import { Snippet } from '../core/backend/snippet-api';
import { PersonalValues, Remember } from '../core/personal-values';
import { Modal } from '../shared/modal';
import { RememberSelect } from '../shared/remember-select';
import { snippetScope, userVariables } from './variables';

/** What the run dialog returns: the values, and where each is remembered. */
export interface RunAnswer {
  values: Record<string, string>;
  where: Record<string, Remember>;
}

/**
 * Asks the values of a snippet's variables before it is sent, or (mine)
 * only this person's remembered values, to see or change them.
 */
@Component({
  selector: 'app-run-dialog',
  imports: [FormsModule, TranslocoDirective, Modal, RememberSelect],
  templateUrl: './run-dialog.html',
  styleUrl: './run-dialog.scss',
})
export class RunDialog {
  private readonly personal = inject(PersonalValues);
  readonly snippet = input.required<Snippet>();
  readonly mine = input(false);
  readonly run = output<RunAnswer>();
  readonly closed = output<void>();

  protected readonly variables = computed(() => userVariables(this.snippet().body));
  protected values: Record<string, string> = {};
  protected where: Record<string, Remember> = {};

  ngOnInit(): void {
    const scope = snippetScope(this.snippet().id);
    for (const v of this.variables()) {
      this.values[v.name] = this.personal.get(v.name, scope) ?? v.choices[0] ?? '';
      const source = this.personal.source(v.name, scope);
      this.where[v.name] = source === 'none' && this.mine() ? 'global' : source;
    }
  }

  protected submit(): void {
    this.run.emit({ values: { ...this.values }, where: { ...this.where } });
  }
}
