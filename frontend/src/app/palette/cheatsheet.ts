import { Component, output } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';

import { Modal } from '../shared/modal';
import { altKey, combo, isMac } from '../sessions/shortcuts';

/** Keyboard shortcuts, in the notation of the current platform. */
@Component({
  selector: 'app-cheatsheet',
  imports: [TranslocoDirective, Modal],
  templateUrl: './cheatsheet.html',
  styleUrl: './cheatsheet.scss',
})
export class Cheatsheet {
  readonly closed = output<void>();
  protected readonly isMac = isMac;
  protected readonly rows = [
    { action: 'palette', keys: combo('K') },
    { action: 'cheatsheet', keys: combo('/') },
    { action: 'togglePanel', keys: combo('L') },
    { action: 'newLocal', keys: combo('T') },
    { action: 'splitRight', keys: combo('D') },
    { action: 'splitDown', keys: combo('E') },
    { action: 'closePane', keys: combo('W') },
    { action: 'broadcast', keys: combo('B') },
    { action: 'focus', keys: altKey + '+← ↑ → ↓' },
    { action: 'search', keys: combo('F') },
    { action: 'maximize', keys: combo('Enter') },
    { action: 'zoom', keys: combo('= / - / 0') },
    { action: 'copyPaste', keys: isMac ? '⌘C / ⌘V' : 'Ctrl+Shift+C / V' },
  ];
}
