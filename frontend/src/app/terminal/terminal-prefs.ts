import { Injectable, signal } from '@angular/core';

import { readPreference, writePreference } from '../core/preferences';
import { AUTO_SCHEME } from './schemes';

export const DEFAULT_FONT_SIZE = 13;
export const FONT_SIZES = [10, 11, 12, 13, 14, 15, 16, 18, 20, 22, 24];
export const SCROLLBACK_SIZES = [1000, 5000, 10000, 50000, 100000];
export type CursorStyle = 'block' | 'bar' | 'underline';

const bool = (key: string, fallback: boolean): boolean => {
  const v = readPreference(key);
  return v === null ? fallback : v === 'true';
};

/** App-wide terminal preferences; groups and hosts may override some. */
@Injectable({ providedIn: 'root' })
export class TerminalPrefs {
  readonly scheme = signal(readPreference('terminal.scheme') ?? AUTO_SCHEME);
  readonly fontSize = signal(Number(readPreference('terminal.fontSize')) || DEFAULT_FONT_SIZE);
  /** Colors log levels, addresses, HTTP codes and UUIDs. */
  readonly builtinHighlights = signal(bool('terminal.builtinHighlights', true));
  readonly cursorStyle = signal<CursorStyle>((readPreference('terminal.cursorStyle') as CursorStyle) || 'block');
  readonly cursorBlink = signal(bool('terminal.cursorBlink', true));
  readonly scrollback = signal(Number(readPreference('terminal.scrollback')) || 10000);
  /** Selecting text copies it. */
  readonly copyOnSelect = signal(bool('terminal.copyOnSelect', false));
  /** Pasting several lines asks first (they would run one by one). */
  readonly confirmPaste = signal(bool('terminal.confirmPaste', true));
  /** Closing a tab with connected panes asks first. */
  readonly confirmClose = signal(bool('terminal.confirmClose', true));
  /** Option/Alt sends Esc+key (Meta) instead of special characters. */
  readonly altIsMeta = signal(bool('terminal.altIsMeta', false));
  /** Backspace sends ^H instead of DEL, for old systems. */
  readonly backspaceCtrlH = signal(bool('terminal.backspaceCtrlH', false));
  /** Exposes the output to screen readers (slower). */
  readonly screenReader = signal(bool('terminal.screenReader', false));
  /** Format of logs started from a pane header. */
  readonly logFormat = signal(readPreference('terminal.logFormat') || 'text');
  /** A bell in a pane that is not visible shows a notification. */
  readonly bellNotify = signal(bool('terminal.bellNotify', true));
  /** Notify when a command running longer than this ends (0 = never). */
  readonly commandNotifySeconds = signal(Number(readPreference('terminal.commandNotifySeconds') ?? 10));
  /** The app's colors follow the terminal scheme. */
  readonly matchApp = signal(bool('terminal.matchApp', false));
  /** Suggests commands as you type, from history and snippets (off by default). */
  readonly suggestions = signal(bool('terminal.suggestions', false));

  setScheme(id: string): void {
    this.scheme.set(id);
    writePreference('terminal.scheme', id);
  }

  setFontSize(size: number): void {
    this.fontSize.set(size);
    writePreference('terminal.fontSize', String(size));
  }

  setBuiltinHighlights(on: boolean): void {
    this.set('builtinHighlights', on);
  }

  /** Changes and remembers one of the simple preferences. */
  set<K extends keyof TerminalPrefs & ('cursorStyle' | 'cursorBlink' | 'scrollback' | 'copyOnSelect' | 'confirmPaste' | 'confirmClose' | 'altIsMeta' | 'backspaceCtrlH' | 'screenReader' | 'bellNotify' | 'builtinHighlights' | 'logFormat' | 'matchApp' | 'commandNotifySeconds' | 'suggestions')>(
    key: K,
    value: ReturnType<TerminalPrefs[K]>,
  ): void {
    (this[key] as unknown as { set(v: unknown): void }).set(value);
    writePreference('terminal.' + key, String(value));
  }
}
