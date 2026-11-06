import { Component, inject, input } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';
import { LucideCircleHelp } from '@lucide/angular';

import { Navigation } from '../layout/navigation';

/** A "?" next to a field or a title: opens the guide at the right place. */
@Component({
  selector: 'app-help-link',
  imports: [TranslocoDirective, LucideCircleHelp],
  templateUrl: './help-link.html',
  styleUrl: './help-link.scss',
})
export class HelpLink {
  readonly topic = input.required<string>();
  readonly anchor = input('');

  private readonly nav = inject(Navigation);

  protected open(ev: Event): void {
    ev.preventDefault();
    ev.stopPropagation();
    this.nav.openGuide(this.topic(), this.anchor());
  }
}
