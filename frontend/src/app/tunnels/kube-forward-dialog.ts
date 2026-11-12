import { Component, computed, inject, input, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective } from '@jsverse/transloco';

import { KubeTarget } from '../core/backend/kube-forward-api';
import { toTranslatableError, TranslatableError } from '../core/backend/errors';
import { Modal } from '../shared/modal';
import { KubeForwardsStore, KubeScope } from './kube-forwards.store';

/**
 * Forwards a local port to a service or a pod of a cluster, reached as
 * the terminal it was opened from reaches it (context, namespace, the
 * environment's tunnel and gcloud profile).
 */
@Component({
  selector: 'app-kube-forward-dialog',
  imports: [FormsModule, TranslocoDirective, Modal],
  templateUrl: './kube-forward-dialog.html',
  styleUrl: './kube-forward-dialog.scss',
})
export class KubeForwardDialog {
  private readonly store = inject(KubeForwardsStore);
  readonly scope = input.required<KubeScope>();
  readonly closed = output<void>();

  protected readonly targets = signal<KubeTarget[]>([]);
  protected readonly loading = signal(true);
  protected readonly busy = signal(false);
  protected readonly error = signal<TranslatableError | null>(null);
  protected filter = '';
  protected readonly query = signal('');
  protected readonly shown = computed(() => {
    const q = this.query().trim().toLowerCase();
    return this.targets().filter((t) => (t.ports ?? []).length > 0 && (!q || t.name.toLowerCase().includes(q)));
  });
  protected chosen: KubeTarget | null = null;
  protected remotePort = 0;
  protected localPort: number | null = null;

  ngOnInit(): void {
    this.store.targets(this.scope()).then(
      (list) => {
        this.targets.set(list);
        this.loading.set(false);
      },
      (err) => {
        this.error.set(toTranslatableError(err));
        this.loading.set(false);
      },
    );
  }

  protected choose(t: KubeTarget): void {
    this.chosen = t;
    this.remotePort = t.ports?.[0] ?? 0;
    // The same port here when it is a high one (8080, 9090…); otherwise a free one.
    this.localPort = this.remotePort >= 1024 ? this.remotePort : null;
  }

  protected async start(): Promise<void> {
    const t = this.chosen;
    if (!t || !this.remotePort) {
      return;
    }
    this.busy.set(true);
    this.error.set(null);
    try {
      await this.store.start({
        selection: this.scope().selection,
        gcloud: this.scope().gcloud,
        kind: t.kind,
        name: t.name,
        remotePort: +this.remotePort,
        localPort: this.localPort ? +this.localPort : 0,
        label: this.scope().label,
      });
      this.closed.emit();
    } catch (err) {
      this.error.set(toTranslatableError(err));
    } finally {
      this.busy.set(false);
    }
  }
}
