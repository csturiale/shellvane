import { TestBed } from '@angular/core/testing';

import { UsageApi } from '../backend/usage-api';
import { UsageStore } from './usage.store';

describe('UsageStore', () => {
  let sent: unknown[];
  let store: UsageStore;

  beforeEach(() => {
    localStorage.clear();
    sent = [];
    TestBed.configureTestingModule({
      providers: [{ provide: UsageApi, useValue: { send: async (r: unknown) => void sent.push(r), payload: async (r: unknown) => r } }],
    });
    store = TestBed.inject(UsageStore);
  });

  it('counts nothing without consent', async () => {
    store.count('pane.split');
    store.command('kubectl get pods');
    expect(await store.preview()).toEqual({ days: 0, features: {}, tools: {} });
  });

  it('counts features and known tools after consent, and forgets them when withdrawn', async () => {
    store.setConsent(true);
    store.count('pane.split');
    store.count('pane.split');
    store.view('tunnels');
    store.view('not-a-view');
    store.command('sudo systemctl restart nginx');
    store.command('./secret.sh --token=x');
    await new Promise((r) => setTimeout(r, 50)); // the tool list loads lazily
    const r = (await store.preview()) as unknown as { features: Record<string, number>; tools: Record<string, number> };
    expect(r.features).toEqual({ 'pane.split': 2, 'view.tunnels': 1 });
    expect(r.tools).toEqual({ systemctl: 1 });
    store.setConsent(false);
    store.setConsent(true);
    expect(((await store.preview()) as unknown as { features: object }).features).toEqual({});
  });

  it('sends once a week', async () => {
    store.setConsent(true);
    store.count('files');
    await store.sendIfDue();
    expect(sent.length).toBe(0);
    const c = JSON.parse(localStorage.getItem('shellvane.usage.counters')!);
    c.since = Date.now() - 8 * 24 * 60 * 60 * 1000;
    localStorage.setItem('shellvane.usage.counters', JSON.stringify(c));
    await store.sendIfDue();
    expect(sent).toEqual([{ days: 8, features: { files: 1 }, tools: {} }]);
    expect(((await store.preview()) as unknown as { features: object }).features).toEqual({});
  });
});
