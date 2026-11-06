import { Component, inject } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';
import { LucideX } from '@lucide/angular';

import { EnvironmentsStore } from './environments.store';

/** Tells that an environment stopped with its last terminal, with an Undo. */
@Component({
  selector: 'app-environment-notice',
  imports: [TranslocoDirective, LucideX],
  templateUrl: './environment-notice.html',
  styleUrl: './environment-notice.scss',
})
export class EnvironmentNotice {
  protected readonly store = inject(EnvironmentsStore);
}
