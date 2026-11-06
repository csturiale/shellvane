import { TestBed } from '@angular/core/testing';
import { TranslocoTestingModule } from '@jsverse/transloco';
import { provideTranslocoMessageformat } from '@jsverse/transloco-messageformat';

import { HostApi } from '../core/backend/host-api';
import { FakeHostApi, codedError } from '../testing/fake-host-api';
import { HostsPanel } from './hosts-panel';
import { HostsStore } from './hosts.store';
import enTranslations from '../../i18n/en.json';

describe('HostsPanel', () => {
  let api: FakeHostApi;

  beforeEach(async () => {
    localStorage.clear();
    api = new FakeHostApi();
    await TestBed.configureTestingModule({
      imports: [
        HostsPanel,
        TranslocoTestingModule.forRoot({
          langs: { en: enTranslations },
          translocoConfig: { availableLangs: ['en'], defaultLang: 'en' },
          preloadLangs: true,
        }),
      ],
      providers: [{ provide: HostApi, useValue: api }, provideTranslocoMessageformat()],
    }).compileComponents();
  });

  async function render() {
    const fixture = TestBed.createComponent(HostsPanel);
    await fixture.whenStable();
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  const rowNames = (el: HTMLElement) =>
    [...el.querySelectorAll('.row .name')].map((n) => n.firstChild?.textContent?.trim());

  it('offers to create the first host or group when empty', async () => {
    const { el } = await render();
    expect(el.querySelector('.empty')?.textContent).toContain('Create a group');
    expect([...el.querySelectorAll('.empty button')].map((b) => b.textContent?.trim())).toEqual([
      'New host',
      'New group',
      'Import',
    ]);
  });

  it('renders groups as a collapsible tree', async () => {
    const prod = await api.createGroup({ name: 'Production', parentId: '', color: 'red', settings: {} });
    await api.createHost({ groupId: prod.id, name: '', hostname: 'web-1', notes: '', settings: {}, tags: [], favorite: false });
    await api.createHost({ groupId: '', name: '', hostname: 'bastion', notes: '', settings: {}, tags: [], favorite: false });

    const { fixture, el } = await render();
    expect(rowNames(el)).toEqual(['Production', 'bastion']);
    expect(el.querySelector('.group .count')?.textContent).toBe('1');

    (el.querySelector('.group') as HTMLElement).click();
    await fixture.whenStable();
    expect(rowNames(el)).toEqual(['Production', 'web-1', 'bastion']);
    expect(el.querySelector('.group')?.getAttribute('aria-expanded')).toBe('true');
  });

  it('creates a subgroup and expands its parent', async () => {
    const prod = await api.createGroup({ name: 'Production', parentId: '', color: '', settings: {} });
    const { fixture, el } = await render();

    await TestBed.inject(HostsStore).createGroup({ name: 'web', parentId: prod.id, color: '', settings: {} });
    await fixture.whenStable();
    expect(rowNames(el)).toEqual(['Production', 'web']);
  });

  it('asks for confirmation with the content count before deleting a group', async () => {
    const prod = await api.createGroup({ name: 'Production', parentId: '', color: '', settings: {} });
    const web = await api.createGroup({ name: 'web', parentId: prod.id, color: '', settings: {} });
    await api.createHost({ groupId: web.id, name: '', hostname: 'a', notes: '', settings: {}, tags: [], favorite: false });
    await api.createHost({ groupId: prod.id, name: '', hostname: 'b', notes: '', settings: {}, tags: [], favorite: false });
    const { fixture, el } = await render();

    (el.querySelector('button[aria-label="Delete Production"]') as HTMLButtonElement).click();
    await fixture.whenStable();
    expect(el.querySelector('app-confirm-dialog .message')?.textContent).toContain(
      'Delete “Production” and everything inside it? 1 subgroup and 2 hosts will be deleted.',
    );

    (el.querySelector('app-confirm-dialog button.danger') as HTMLButtonElement).click();
    await fixture.whenStable();
    expect(api.groups).toEqual([]);
    expect(api.hosts).toEqual([]);
    expect(el.querySelector('app-confirm-dialog')).toBeNull();
  });

  it('shows translated errors from the backend', async () => {
    api.moveGroup = () => Promise.reject(codedError('hosts.groupCycle'));
    const g = await api.createGroup({ name: 'a', parentId: '', color: '', settings: {} });
    const { fixture, el } = await render();

    // Same path a drop takes: the panel runs the store action and reports errors.
    const panel = fixture.componentInstance as unknown as { run(action: () => Promise<unknown>): Promise<void> };
    await panel.run(() => TestBed.inject(HostsStore).moveGroup(g.id, g.id));
    fixture.detectChanges();
    expect(el.querySelector('.error')?.textContent).toBe('A group cannot be moved inside itself.');
  });
});
