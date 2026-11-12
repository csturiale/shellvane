import { TestBed } from '@angular/core/testing';

import { SafeMessageFormatTranspiler } from './safe-transpiler';

describe('SafeMessageFormatTranspiler', () => {
  const t = () => TestBed.runInInjectionContext(() => new SafeMessageFormatTranspiler({}));
  const run = (value: string, params: Record<string, unknown>) =>
    t().transpile({ value, params, translation: {}, key: 'k' } as Parameters<SafeMessageFormatTranspiler['transpile']>[0]);

  it("keeps the user's text as written", () => {
    const cmd = `kubectl run -n "\${NS_ARGOCD}" x -- sh -c 'echo {a} #1'`;
    expect(run('The text has {{lines}} lines:\n\n{{preview}}', { lines: 9, preview: cmd })).toBe(`The text has 9 lines:\n\n${cmd}`);
  });

  it('still does ICU plurals', () => {
    expect(run('{count, plural, one {# host} other {# hosts}} for {{name}}', { count: 2, name: 'web {prod}' })).toBe('2 hosts for web {prod}');
  });
});
