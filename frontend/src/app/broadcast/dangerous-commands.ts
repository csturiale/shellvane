import { Injectable, computed, signal } from '@angular/core';

import { readPreference, writePreference } from '../core/preferences';
import { DEFAULT_DANGEROUS, compileDangerous, isDangerous } from '../snippets/variables';

const KEY = 'broadcast.dangerous';

/** Regular expressions of commands that ask for confirmation when sent to several panes. */
@Injectable({ providedIn: 'root' })
export class DangerousCommands {
  readonly patterns = signal<string[]>(load());
  private readonly compiled = computed(() => compileDangerous(this.patterns()));
  /** Patterns that are not valid regular expressions. */
  readonly invalid = computed(() => this.patterns().filter((p) => compileDangerous([p]).length === 0));

  test(command: string): boolean {
    return isDangerous(command, this.compiled());
  }

  set(patterns: string[]): void {
    const list = patterns.map((p) => p.trim()).filter(Boolean);
    this.patterns.set(list);
    writePreference(KEY, JSON.stringify(list));
  }

  reset(): void {
    this.set([...DEFAULT_DANGEROUS]);
  }
}

function load(): string[] {
  try {
    const v = JSON.parse(readPreference(KEY) ?? 'null') as unknown;
    if (Array.isArray(v) && v.every((x) => typeof x === 'string')) {
      return v;
    }
  } catch {
    // Fall back to the defaults.
  }
  return [...DEFAULT_DANGEROUS];
}
