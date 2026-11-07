import { TestBed } from '@angular/core/testing';

import { UpdateApi, UpdateInfo } from '../core/backend/update-api';
import { UpdatesStore } from './updates.store';

const info = (over: Partial<UpdateInfo> = {}): UpdateInfo => ({
  current: '1.0.0',
  latest: '1.1.0',
  available: true,
  notes: {},
  page: '',
  canInstall: true,
  enabled: true,
  noRelease: false,
  blocked: '',
  ...over,
});

describe('UpdatesStore notice', () => {
  let store: UpdatesStore;
  let next: UpdateInfo;

  beforeEach(() => {
    localStorage.clear();
    next = info();
    TestBed.configureTestingModule({
      providers: [{ provide: UpdateApi, useValue: { check: async () => next, install: async () => undefined, restart: async () => undefined } }],
    });
    store = TestBed.inject(UpdatesStore);
  });

  it('offers an available update, not an absent one', async () => {
    await store.check();
    expect(store.prompt()).toBe(true);
    next = info({ available: false });
    await store.check();
    expect(store.prompt()).toBe(false);
  });

  it('skips a version until the next one', async () => {
    await store.check();
    store.skip();
    expect(store.prompt()).toBe(false);
    next = info({ latest: '1.2.0' });
    await store.check();
    expect(store.prompt()).toBe(true);
  });

  it('stays quiet when snoozed or turned off, and offers the restart after installing', async () => {
    await store.check();
    store.snooze();
    expect(store.prompt()).toBe(false);
    localStorage.clear();
    const fresh = TestBed.runInInjectionContext(() => new UpdatesStore());
    await fresh.check();
    fresh.setNotify(false);
    expect(fresh.prompt()).toBe(false);
    await fresh.install();
    expect(fresh.prompt()).toBe(true);
    fresh.dismiss();
    expect(fresh.prompt()).toBe(false);
  });
});
