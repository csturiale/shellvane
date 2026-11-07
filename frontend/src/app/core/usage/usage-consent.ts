import { Component, inject } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';

import { Modal } from '../../shared/modal';
import { openUrl } from '../browser';
import { SITE_URL } from '../site';
import { UsageStore } from './usage.store';

/** The question asked once: share anonymous usage statistics or not. */
@Component({
  selector: 'app-usage-consent',
  imports: [TranslocoDirective, Modal],
  templateUrl: './usage-consent.html',
  styleUrl: './usage-consent.scss',
})
export class UsageConsent {
  protected readonly usage = inject(UsageStore);

  protected privacy(): void {
    void openUrl(`${SITE_URL}/privacy/#usage`);
  }
}
