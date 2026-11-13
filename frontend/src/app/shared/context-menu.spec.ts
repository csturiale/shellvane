import { TestBed } from '@angular/core/testing';
import { TranslocoTestingModule } from '@jsverse/transloco';

import { ContextMenu } from './context-menu';
import { ContextMenuService, submenu } from './context-menu.service';

describe('ContextMenu', () => {
  it('opens a second level and runs the entry chosen there', async () => {
    TestBed.configureTestingModule({
      imports: [ContextMenu, TranslocoTestingModule.forRoot({ langs: { en: {} }, translocoConfig: { defaultLang: 'en', availableLangs: ['en'] } })],
    });
    const f = TestBed.createComponent(ContextMenu);
    const service = TestBed.inject(ContextMenuService);
    const ran: string[] = [];
    service.open(new MouseEvent('contextmenu', { clientX: 10, clientY: 10 }), [
      { label: 'edit', action: () => ran.push('edit') },
      submenu('share', [
        { label: 'ops', action: () => ran.push('ops') },
        { label: 'dev', action: () => ran.push('dev') },
      ]),
    ]);
    f.detectChanges();
    await f.whenStable();
    const parent = f.nativeElement.querySelector('button.parent') as HTMLButtonElement;
    expect(parent.getAttribute('aria-expanded')).toBe('false');
    parent.click(); // opens, runs nothing
    f.detectChanges();
    expect(ran).toEqual([]);
    expect(service.menu()).not.toBeNull();
    const sub = f.nativeElement.querySelector('.context-menu.sub') as HTMLElement;
    expect([...sub.querySelectorAll('button')].map((b) => b.textContent?.trim())).toEqual(['ops', 'dev']);
    (sub.querySelectorAll('button')[1] as HTMLButtonElement).click();
    f.detectChanges();
    expect(ran).toEqual(['dev']);
    expect(service.menu()).toBeNull();
  });
});
