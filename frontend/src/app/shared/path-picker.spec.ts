import { TestBed } from '@angular/core/testing';
import { TranslocoTestingModule } from '@jsverse/transloco';

import { DialogApi } from '../core/backend/dialog-api';
import { PathPicker } from './path-picker';

describe('PathPicker', () => {
  let asked: { kind: string; current: string }[];
  let answer: string;

  beforeEach(() => {
    asked = [];
    answer = '';
    TestBed.configureTestingModule({
      imports: [PathPicker, TranslocoTestingModule.forRoot({ langs: { en: {} }, translocoConfig: { defaultLang: 'en', availableLangs: ['en'] } })],
      providers: [
        {
          provide: DialogApi,
          useValue: {
            chooseFile: async (_t: string, current: string) => (asked.push({ kind: 'file', current }), answer),
            chooseFolder: async (_t: string, current: string) => (asked.push({ kind: 'folder', current }), answer),
          },
        },
      ],
    });
  });

  it('opens the chooser near the typed path and emits the one chosen', async () => {
    const f = TestBed.createComponent(PathPicker);
    f.componentRef.setInput('kind', 'folder');
    f.componentRef.setInput('current', '~/work');
    const picked: string[] = [];
    f.componentInstance.picked.subscribe((p) => picked.push(p));
    f.detectChanges();
    answer = '/Users/me/work/runbooks';
    (f.nativeElement.querySelector('button') as HTMLButtonElement).click();
    await new Promise((r) => setTimeout(r));
    expect(asked).toEqual([{ kind: 'folder', current: '~/work' }]);
    expect(picked).toEqual(['/Users/me/work/runbooks']);
  });

  it('emits nothing when the chooser is cancelled', async () => {
    const f = TestBed.createComponent(PathPicker);
    const picked: string[] = [];
    f.componentInstance.picked.subscribe((p) => picked.push(p));
    f.detectChanges();
    (f.nativeElement.querySelector('button') as HTMLButtonElement).click();
    await new Promise((r) => setTimeout(r));
    expect(asked).toEqual([{ kind: 'file', current: '' }]);
    expect(picked).toEqual([]);
  });
});
