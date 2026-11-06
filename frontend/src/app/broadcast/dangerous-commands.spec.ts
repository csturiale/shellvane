import { TestBed } from '@angular/core/testing';

import { DangerousCommands } from './dangerous-commands';

describe('DangerousCommands', () => {
  beforeEach(() => localStorage.clear());

  it('uses the defaults, then the configured patterns', () => {
    const d = TestBed.inject(DangerousCommands);
    expect(d.test('sudo rm -rf /')).toBe(true);
    expect(d.test('DROP TABLE users')).toBe(true);
    d.set(['kubectl\\s+delete', '(bad', '']);
    expect(d.patterns()).toEqual(['kubectl\\s+delete', '(bad']);
    expect(d.invalid()).toEqual(['(bad']);
    expect(d.test('kubectl delete pod x')).toBe(true);
    expect(d.test('rm -rf /')).toBe(false);
    d.reset();
    expect(d.test('rm -rf /')).toBe(true);
  });
});
