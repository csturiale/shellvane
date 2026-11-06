import { Literals } from './literals';

describe('Literals', () => {
  it('puts literal text back after translation', () => {
    const l = new Literals({ example: '{{name}}' });
    const translated = `Write ${l.params['example']} here`;
    expect(l.fill(translated)).toBe('Write {{name}} here');
  });
});
