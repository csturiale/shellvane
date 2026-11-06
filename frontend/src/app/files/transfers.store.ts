import { Injectable, computed, inject, signal } from '@angular/core';

import { FilesApi, TransferStatus } from '../core/backend/files-api';

@Injectable({ providedIn: 'root' })
export class TransfersStore {
  private readonly api = inject(FilesApi);
  readonly transfers = signal<TransferStatus[]>([]);
  readonly active = computed(() => this.transfers().filter((t) => t.state === 'queued' || t.state === 'running').length);
  /** Bumped when a transfer finishes, so panes can refresh their listing. */
  readonly completed = signal(0);

  constructor() {
    try {
      this.api.onTransfer((s) => {
        this.transfers.update((list) => {
          const i = list.findIndex((t) => t.id === s.id);
          return i < 0 ? [...list, s] : list.map((t) => (t.id === s.id ? s : t));
        });
        if (s.state === 'done') {
          this.completed.update((n) => n + 1);
        }
      });
    } catch {
      // No event bus outside the desktop app.
    }
  }

  cancel(id: string): Promise<void> {
    return this.api.cancel(id);
  }

  async clearFinished(): Promise<void> {
    await this.api.clearFinished();
    this.transfers.update((list) => list.filter((t) => t.state === 'queued' || t.state === 'running'));
  }
}
