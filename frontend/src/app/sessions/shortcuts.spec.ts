import { macLineEdit } from './shortcuts';

const key = (k: string, mods: Partial<KeyboardEvent> = {}) => ({ type: 'keydown', key: k, altKey: false, metaKey: false, ctrlKey: false, shiftKey: false, ...mods }) as KeyboardEvent;

describe('macLineEdit', () => {
  it('sends the shell its word and line moves on macOS', () => {
    expect(macLineEdit(key('ArrowLeft', { altKey: true }), true)).toBe('\x1bb');
    expect(macLineEdit(key('ArrowRight', { altKey: true }), true)).toBe('\x1bf');
    expect(macLineEdit(key('ArrowLeft', { metaKey: true }), true)).toBe('\x01');
    expect(macLineEdit(key('ArrowRight', { metaKey: true }), true)).toBe('\x05');
    expect(macLineEdit(key('Backspace', { altKey: true }), true)).toBe('\x1b\x7f');
    expect(macLineEdit(key('Backspace', { metaKey: true }), true)).toBe('\x15');
  });

  it('leaves other keys, ⌘⌥ (pane moves) and other systems alone', () => {
    expect(macLineEdit(key('ArrowLeft'), true)).toBeNull();
    expect(macLineEdit(key('ArrowLeft', { altKey: true, metaKey: true }), true)).toBeNull();
    expect(macLineEdit(key('ArrowLeft', { altKey: true, shiftKey: true }), true)).toBeNull();
    expect(macLineEdit(key('a', { metaKey: true }), true)).toBeNull();
    expect(macLineEdit(key('ArrowLeft', { altKey: true }), false)).toBeNull();
  });
});
