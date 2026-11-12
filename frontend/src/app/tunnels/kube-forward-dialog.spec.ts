import { TestBed } from '@angular/core/testing';
import { TranslocoTestingModule } from '@jsverse/transloco';

import { KubeForwardApi, KubeForwardSpec } from '../core/backend/kube-forward-api';
import { KubeForwardDialog } from './kube-forward-dialog';

describe('KubeForwardDialog', () => {
  it('lists services with ports and forwards the one chosen', async () => {
    const started: KubeForwardSpec[] = [];
    TestBed.configureTestingModule({
      imports: [KubeForwardDialog, TranslocoTestingModule.forRoot({ langs: { en: {} }, translocoConfig: { defaultLang: 'en', availableLangs: ['en'] } })],
      providers: [
        {
          provide: KubeForwardApi,
          useValue: {
            targets: async () => [
              { kind: 'service', name: 'argocd-server', ports: [80, 443] },
              { kind: 'pod', name: 'job-x', ports: [] },
              { kind: 'pod', name: 'grafana-0', ports: [3000] },
            ],
            start: async (spec: KubeForwardSpec) => (started.push(spec), { ...spec, id: 'f1', state: 'starting' }),
            list: async () => [],
          },
        },
      ],
    });
    const f = TestBed.createComponent(KubeForwardDialog);
    const closed: unknown[] = [];
    f.componentInstance.closed.subscribe(() => closed.push(1));
    f.componentRef.setInput('scope', { selection: { context: 'shop-prd', namespace: 'argocd', route: { proxyUrl: 'socks5://127.0.0.1:10821', server: '' } }, gcloud: 'prd', label: 'GKE shop prd' });
    f.detectChanges();
    await f.whenStable();
    f.detectChanges();
    const options = [...f.nativeElement.querySelectorAll('.target')] as HTMLButtonElement[];
    expect(options.map((o) => o.querySelector('.name')?.textContent + ' ' + o.querySelector('.ports')?.textContent)).toEqual(['argocd-server 80, 443', 'grafana-0 3000']);

    options[1].click(); // a high port: the same one here
    f.detectChanges();
    const c = f.componentInstance as unknown as { localPort: number | null; remotePort: number; start(): Promise<void>; choose(t: unknown): void };
    expect(c.localPort).toBe(3000);
    c.choose({ kind: 'service', name: 'argocd-server', ports: [80, 443] }); // a low port: any free one
    expect(c.localPort).toBeNull();
    c.remotePort = 443;
    await c.start();
    expect(started).toEqual([
      {
        selection: { context: 'shop-prd', namespace: 'argocd', route: { proxyUrl: 'socks5://127.0.0.1:10821', server: '' } },
        gcloud: 'prd',
        kind: 'service',
        name: 'argocd-server',
        remotePort: 443,
        localPort: 0,
        label: 'GKE shop prd',
      },
    ]);
    expect(closed).toHaveLength(1);
  });
});
