import { formatBytes } from './format';

describe('formatBytes', () => {
  it('uses the language conventions', () => {
    expect(formatBytes(512, 'en')).toBe('512 byte');
    expect(formatBytes(1_234_567, 'en')).toBe('1.2 MB');
    expect(formatBytes(1_234_567, 'it')).toBe('1,2 MB');
  });
});
