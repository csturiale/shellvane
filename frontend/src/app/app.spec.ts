import { TestBed } from '@angular/core/testing';
import { TranslocoTestingModule } from '@jsverse/transloco';

import { App } from './app';
import { BackendService } from './core/backend/backend.service';
import { HostApi } from './core/backend/host-api';
import { LanguageService } from './core/i18n/language.service';
import { FakeHostApi } from './testing/fake-host-api';
import enTranslations from '../i18n/en.json';
import itTranslations from '../i18n/it.json';

class FakeBackend {
  readonly connected = () => false;
  info = () => Promise.resolve(null);
  setLanguage = () => Promise.resolve();
}

describe('App', () => {
  beforeEach(async () => {
    localStorage.clear();
    await TestBed.configureTestingModule({
      imports: [
        App,
        TranslocoTestingModule.forRoot({
          langs: { en: enTranslations, it: itTranslations },
          translocoConfig: { availableLangs: ['en', 'it'], defaultLang: 'en', reRenderOnLangChange: true },
          preloadLangs: true,
        }),
      ],
      providers: [
        { provide: BackendService, useClass: FakeBackend },
        { provide: HostApi, useValue: new FakeHostApi() },
      ],
    }).compileComponents();
  });

  async function render() {
    const fixture = TestBed.createComponent(App);
    await fixture.whenStable();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  it('shows one section at a time, starting with hosts', async () => {
    const { el } = await render();
    expect(el.querySelectorAll('.panel')).not.toHaveLength(0);
    expect(el.querySelector('.panel h2')?.textContent?.trim()).toBe('Hosts');
    expect(el.querySelector('app-activity-bar [aria-current="page"]')?.getAttribute('aria-label')).toBe('Hosts');
  });

  it('switches section from the activity bar', async () => {
    const { fixture, el } = await render();
    (el.querySelector('app-activity-bar button[aria-label="Tunnels"]') as HTMLButtonElement).click();
    await fixture.whenStable();
    expect(el.querySelector('.panel h2')?.textContent?.trim()).toBe('Tunnels');
  });

  it('opens settings in the workspace and hides the side panel', async () => {
    const { fixture, el } = await render();
    (el.querySelector('app-activity-bar button[aria-label="Settings"]') as HTMLButtonElement).click();
    await fixture.whenStable();
    expect(el.querySelector('.panel')).toBeNull();
    expect(el.querySelector('app-settings-page h1')?.textContent).toBe('Settings');
  });

  it('shows the current language in settings', async () => {
    TestBed.inject(LanguageService).set('it');
    const { fixture, el } = await render();
    (el.querySelector('app-activity-bar button[aria-label="Impostazioni"]') as HTMLButtonElement).click();
    await fixture.whenStable();
    expect((el.querySelector('#language') as HTMLSelectElement).value).toBe('it');
  });

  it('switches language at runtime', async () => {
    const { fixture, el } = await render();
    TestBed.inject(LanguageService).set('it');
    await fixture.whenStable();
    expect(el.querySelector('.panel h2')?.textContent?.trim()).toBe('Host');
    expect(el.querySelector('.statusbar')?.textContent).toContain("In esecuzione fuori dall'app desktop");
    expect(document.documentElement.lang).toBe('it');
  });
});
