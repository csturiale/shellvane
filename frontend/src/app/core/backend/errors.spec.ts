import { toTranslatableError } from './errors';

describe('toTranslatableError', () => {
  it('maps a coded Go error to a translation key', () => {
    const err = Object.assign(new Error('notFound (id=h1)'), {
      cause: { code: 'notFound', params: { id: 'h1' } },
    });
    expect(toTranslatableError(err)).toEqual({ key: 'errors.notFound', params: { id: 'h1' }, detail: undefined });
  });

  it('treats anything else as internal', () => {
    expect(toTranslatableError(new Error('boom'))).toEqual({ key: 'errors.internal', params: {}, detail: 'boom' });
    expect(toTranslatableError('weird').key).toBe('errors.internal');
  });
});
