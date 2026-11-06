import { looksLikeTarget } from './new-tab-menu';

describe('looksLikeTarget', () => {
  it('recognizes addresses but not searches', () => {
    for (const t of ['root@10.0.0.1', 'deploy@web.example.org', 'db.internal:2222', 'telnet://switch', 'serial://COM3']) {
      expect([t, looksLikeTarget(t)]).toEqual([t, true]);
    }
    for (const t of ['prod web', 'web', 'kube', '']) {
      expect([t, looksLikeTarget(t)]).toEqual([t, false]);
    }
  });
});
