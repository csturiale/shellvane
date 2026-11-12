import { Component, input, model } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';

import { Remember } from '../core/personal-values';

/** Where a typed value is remembered: not at all, everywhere, or only for this item. */
@Component({
  selector: 'app-remember-select',
  imports: [TranslocoDirective],
  templateUrl: './remember-select.html',
  styleUrl: './remember-select.scss',
})
export class RememberSelect {
  readonly value = model<Remember>('none');
  /** Offers "don't remember" (environments' personal variables always are). */
  readonly allowNone = input(true);
  readonly label = input('');

  protected onChange(ev: Event): void {
    this.value.set((ev.target as HTMLSelectElement).value as Remember);
  }
}
