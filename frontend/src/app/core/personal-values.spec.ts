import { TestBed } from '@angular/core/testing';

import { PersonalValues } from './personal-values';

describe('PersonalValues', () => {
  let pv: PersonalValues;

  beforeEach(() => {
    localStorage.clear();
    TestBed.configureTestingModule({});
    pv = TestBed.inject(PersonalValues);
  });

  afterEach(() => localStorage.clear());

  it('fills a name everywhere, with an item keeping its own value', () => {
    pv.remember('googleEmail', 'me@corp.example', 'global', 'env:a');
    expect(pv.resolve(['googleEmail', 'other'], 'snippet:x')).toEqual({ googleEmail: 'me@corp.example' });
    pv.remember('googleEmail', 'me@client.example', 'here', 'env:b');
    expect(pv.get('googleEmail', 'env:b')).toBe('me@client.example');
    expect(pv.source('googleEmail', 'env:b')).toBe('here');
    expect(pv.get('googleEmail', 'env:a')).toBe('me@corp.example');
    expect(pv.source('googleEmail', 'env:a')).toBe('global');
    expect(pv.source('nothing')).toBe('none');
  });

  it('drops an item\'s own value when remembered everywhere again', () => {
    pv.remember('user', 'a123', 'here', 'env:a');
    pv.remember('user', 'a456', 'global', 'env:a');
    expect(pv.source('user', 'env:a')).toBe('global');
    expect(pv.get('user', 'env:a')).toBe('a456');
  });

  it('remembers nothing for "none" and forgets a scope', () => {
    pv.remember('ns', 'dev', 'none', 'procedure:p');
    expect(pv.get('ns', 'procedure:p')).toBeUndefined();
    pv.remember('ns', 'dev', 'here', 'procedure:p');
    pv.clearScope('procedure:p');
    expect(pv.get('ns', 'procedure:p')).toBeUndefined();
  });

  it('keeps values across restarts and ignores damaged storage', () => {
    pv.setGlobal('safe', 'ADM_a123');
    expect(TestBed.runInInjectionContext(() => new PersonalValues()).global()).toEqual({ safe: 'ADM_a123' });
    localStorage.setItem('shellvane.personal.values', '[1,2');
    expect(new PersonalValues().global()).toEqual({});
  });
});
