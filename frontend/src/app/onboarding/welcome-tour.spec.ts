import { TestBed } from '@angular/core/testing';
import { TranslocoTestingModule } from '@jsverse/transloco';
import { provideTranslocoMessageformat } from '@jsverse/transloco-messageformat';

import { ImportApi } from '../core/backend/import-api';
import { VaultApi } from '../core/backend/vault-api';
import { Navigation } from '../layout/navigation';
import { WelcomeTour } from './welcome-tour';
import enTranslations from '../../i18n/en.json';

describe('WelcomeTour', () => {
  let state: string;
  let setupWith: string | null;
  let keychain: boolean;

  beforeEach(async () => {
    localStorage.clear();
    state = 'notSetUp';
    setupWith = null;
    keychain = false;
    const vaultApi = {
      onState: () => undefined,
      status: async () => state,
      hasRecoveryKey: async () => false,
      hasKeychainUnlock: async () => keychain,
      biometric: async () => ({ kind: '', available: false, required: false }),
      credentials: async () => [],
      setAutoLock: async () => undefined,
      setup: async (pw: string) => {
        setupWith = pw;
        state = 'unlocked';
      },
      enableKeychainUnlock: async () => {
        keychain = true;
      },
    };
    const importApi = {
      sources: async () => [
        { id: 'openssh', kind: 'file', path: '/home/me/.ssh/config', found: true },
        { id: 'putty', kind: 'registry', path: '', found: false },
      ],
    };
    await TestBed.configureTestingModule({
      imports: [
        WelcomeTour,
        TranslocoTestingModule.forRoot({
          langs: { en: enTranslations },
          translocoConfig: { availableLangs: ['en'], defaultLang: 'en' },
          preloadLangs: true,
        }),
      ],
      providers: [
        { provide: VaultApi, useValue: vaultApi },
        { provide: ImportApi, useValue: importApi },
        provideTranslocoMessageformat(),
      ],
    }).compileComponents();
  });

  async function settle(fixture: { whenStable(): Promise<unknown>; detectChanges(): void }) {
    for (let i = 0; i < 3; i++) {
      await fixture.whenStable();
      fixture.detectChanges();
    }
  }

  const button = (el: HTMLElement, label: string) =>
    [...el.querySelectorAll('button')].find(
      (b) => b.textContent?.trim() === label,
    ) as HTMLButtonElement;

  it('creates the vault with the keychain option, lists found tools and opens the import', async () => {
    const fixture = TestBed.createComponent(WelcomeTour);
    const el = fixture.nativeElement as HTMLElement;
    let closed = 0;
    fixture.componentInstance.closed.subscribe(() => closed++);
    await settle(fixture);
    expect(el.textContent).toContain('Welcome to Shellvane');

    button(el, 'Next').click();
    await settle(fixture);
    expect(el.textContent).toContain('Create the vault');
    const inputs = el.querySelectorAll<HTMLInputElement>('input[type=password]');
    for (const i of Array.from(inputs)) {
      i.value = 'a long master password';
      i.dispatchEvent(new Event('input'));
    }
    el.querySelector<HTMLInputElement>('input[name=keychain]')!.click();
    await settle(fixture);
    el.querySelector<HTMLFormElement>('form#tour-vault')!.dispatchEvent(new Event('submit'));
    await settle(fixture);
    expect(setupWith).toBe('a long master password');
    expect(keychain).toBe(true);
    expect(el.textContent).toContain('Bring in your hosts');
    const found = el.querySelector('.sources')?.textContent ?? '';
    expect(found).toContain('OpenSSH');
    expect(found).toContain('/home/me/.ssh/config');
    expect(found).not.toContain('PuTTY');

    const nav = TestBed.inject(Navigation);
    button(el, 'Import hosts').click();
    expect(closed).toBe(1);
    expect(nav.view()).toBe('hosts');
    expect(nav.request()).toBe('import');
  });

  it('refuses mismatched passwords', async () => {
    const fixture = TestBed.createComponent(WelcomeTour);
    const el = fixture.nativeElement as HTMLElement;
    await settle(fixture);
    button(el, 'Next').click();
    await settle(fixture);
    const [a, b] = Array.from(el.querySelectorAll<HTMLInputElement>('input[type=password]'));
    a.value = 'one long password';
    b.value = 'another long password';
    a.dispatchEvent(new Event('input'));
    b.dispatchEvent(new Event('input'));
    await settle(fixture);
    el.querySelector<HTMLFormElement>('form#tour-vault')!.dispatchEvent(new Event('submit'));
    await settle(fixture);
    expect(setupWith).toBeNull();
    expect(el.querySelector('.error')).not.toBeNull();
  });
});
