import { Injectable, signal } from '@angular/core';

import { readPreference, writePreference } from '../core/preferences';

/**
 * Follows what the person types on the shell's command line, so that a
 * finished command can be remembered and the current one completed.
 *
 * It is deliberately strict: as soon as the line may differ from what was
 * typed (cursor keys, Tab completion, history, Ctrl shortcuts) it stops
 * following it until the next Enter, so it never suggests on a line it
 * does not know.
 */
export class LineTracker {
  /** The line typed since the last Enter. */
  line = '';
  /** Whether line is exactly what the shell has (nothing edited it). */
  clean = true;

  /** Adds typed data; returns the lines finished with Enter while clean. */
  feed(data: string): string[] {
    const done: string[] = [];
    for (let i = 0; i < data.length; i++) {
      const c = data[i];
      if (c === '\x1b') {
        // An escape sequence (arrows, Home, function keys…): the cursor or
        // the line may change in ways not followed here.
        this.clean = false;
        i = skipEscape(data, i);
      } else if (c === '\r' || c === '\n') {
        if (this.clean && this.line.trim()) {
          done.push(this.line);
        }
        this.line = '';
        this.clean = true;
      } else if (c === '\x7f' || c === '\x08') {
        this.line = this.line.slice(0, -1);
      } else if (c === '\x15') {
        this.line = ''; // Ctrl+U clears the line
      } else if (c === '\x03') {
        this.line = ''; // Ctrl+C abandons it
        this.clean = true;
      } else if (c < ' ') {
        this.clean = false; // Tab, Ctrl+R, Ctrl+A, Ctrl+W…
      } else {
        this.line += c;
      }
    }
    return done;
  }

  reset(): void {
    this.line = '';
    this.clean = true;
  }
}

/** Index of the last character of the escape sequence starting at i. */
function skipEscape(data: string, i: number): number {
  const next = data[i + 1];
  if (next === '[' || next === 'O') {
    let j = i + 2;
    while (j < data.length && !/[A-Za-z~]/.test(data[j])) {
      j++;
    }
    return Math.min(j, data.length - 1);
  }
  return Math.min(i + 1, data.length - 1);
}

/**
 * Commands that look like they carry a secret are never remembered, nor
 * those starting with a space (the shells' own "do not keep this" habit).
 */
const SECRET = /(pass(word|wd)?|secret|token|api[_-]?key|auth|bearer|private[_-]?key)\w*\s*[=:]|--?(password|passwd|token|secret)[= ]\S|\b(mysql|mariadb|mysqldump|mysqladmin)\b.*\s-p\S|(AKIA|ASIA)[A-Z0-9]{16}|gh[pousr]_[A-Za-z0-9]{20,}|sk-[A-Za-z0-9_-]{16,}|xox[abp]-|BEGIN [A-Z ]*PRIVATE KEY/i;

export function keepable(command: string): boolean {
  return !!command.trim() && !command.startsWith(' ') && command.length <= 1000 && !SECRET.test(command);
}

const PER_SCOPE = 300;
const MAX_SCOPES = 200;
const KEY = 'terminal.commandHistory';

/**
 * Commands run in terminals, kept on this computer by connection (a host,
 * the local shell…), newest first, to suggest them again.
 */
@Injectable({ providedIn: 'root' })
export class CommandHistory {
  private data: Record<string, string[]> = load();
  /** Changes when the history does (for the settings' count). */
  readonly size = signal(count(this.data));

  remember(scope: string, command: string): void {
    const cmd = command.trimEnd();
    if (!keepable(cmd)) {
      return;
    }
    const list = (this.data[scope] ?? []).filter((c) => c !== cmd);
    list.unshift(cmd);
    this.data[scope] = list.slice(0, PER_SCOPE);
    const scopes = Object.keys(this.data);
    if (scopes.length > MAX_SCOPES) {
      delete this.data[scopes[0]];
    }
    this.save();
  }

  /**
   * The best completion of prefix: a command used on this connection, then
   * on any other, then a snippet; '' when none.
   */
  suggest(scope: string, prefix: string, snippets: readonly string[] = []): string {
    if (!prefix.trim()) {
      return '';
    }
    const fits = (c: string) => c.length > prefix.length && c.startsWith(prefix);
    const own = this.data[scope]?.find(fits);
    if (own) {
      return own;
    }
    for (const [s, list] of Object.entries(this.data)) {
      if (s !== scope) {
        const other = list.find(fits);
        if (other) {
          return other;
        }
      }
    }
    return snippets.find(fits) ?? '';
  }

  clear(): void {
    this.data = {};
    this.save();
  }

  private save(): void {
    writePreference(KEY, JSON.stringify(this.data));
    this.size.set(count(this.data));
  }
}

function load(): Record<string, string[]> {
  try {
    const raw = JSON.parse(readPreference(KEY) ?? '{}') as unknown;
    return raw && typeof raw === 'object' ? (raw as Record<string, string[]>) : {};
  } catch {
    return {};
  }
}

function count(data: Record<string, string[]>): number {
  return Object.values(data).reduce((n, l) => n + l.length, 0);
}
