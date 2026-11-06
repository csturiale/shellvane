import { bit, parseOctal, specialBit, toOctal, toSymbolic } from './permissions';

describe('permissions', () => {
  it('computes bits', () => {
    expect(bit('owner', 'read')).toBe(0o400);
    expect(bit('group', 'write')).toBe(0o020);
    expect(bit('others', 'execute')).toBe(0o001);
    expect(specialBit('setuid')).toBe(0o4000);
    expect(specialBit('sticky')).toBe(0o1000);
  });

  it('formats and parses octal', () => {
    expect(toOctal(0o755)).toBe('0755');
    expect(toOctal(0o4711)).toBe('4711');
    expect(parseOctal('644')).toBe(0o644);
    expect(parseOctal('2775')).toBe(0o2775);
    expect(parseOctal('9')).toBeNull();
    expect(parseOctal('77777')).toBeNull();
  });

  it('formats symbolic modes like ls', () => {
    expect(toSymbolic(0o755)).toBe('rwxr-xr-x');
    expect(toSymbolic(0o640)).toBe('rw-r-----');
    expect(toSymbolic(0o4755)).toBe('rwsr-xr-x');
    expect(toSymbolic(0o1777)).toBe('rwxrwxrwt');
    expect(toSymbolic(0o1666)).toBe('rw-rw-rwT');
  });
});
