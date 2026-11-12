import { Injectable, signal } from '@angular/core';

import { readPreference, writePreference } from './preferences';

/** Where a value is remembered: not at all, for every item, or for one item only. */
export type Remember = 'none' | 'global' | 'here';

/**
 * This person's values (a sign-in email, a vault user and safe), kept on
 * this computer only and never exported or shared. A value is stored by
 * name and fills every environment, snippet and procedure that uses that
 * name; one item can keep its own value instead ("only here"), which wins.
 * Scopes name the item: env:<id>, snippet:<id>, procedure:<source id>.
 */
@Injectable({ providedIn: 'root' })
export class PersonalValues {
  /** Values by name for every item. */
  readonly global = signal<Record<string, string>>(read('personal.values'));
  /** Bumped on every change of a scope, for views that show them. */
  readonly version = signal(0);

  /** The values of one item only. */
  scoped(scope: string): Record<string, string> {
    return read(`personal.scoped.${scope}`);
  }

  /** The value of name for the item: its own first, then the global one. */
  get(name: string, scope?: string): string | undefined {
    const own = scope ? this.scoped(scope)[name] : undefined;
    return own ?? this.global()[name];
  }

  /** Known values of names for the item (unknown names are left out). */
  resolve(names: string[], scope?: string): Record<string, string> {
    const own = scope ? this.scoped(scope) : {};
    const all = this.global();
    const out: Record<string, string> = {};
    for (const n of names) {
      const v = own[n] ?? all[n];
      if (v !== undefined && v !== '') {
        out[n] = v;
      }
    }
    return out;
  }

  /** Where the item's value of name comes from ('none': not known). */
  source(name: string, scope?: string): Remember {
    if (scope && this.scoped(scope)[name] !== undefined) {
      return 'here';
    }
    return this.global()[name] !== undefined ? 'global' : 'none';
  }

  /**
   * Remembers a value. 'global' also drops the item's own value, so the
   * global one applies there again; 'none' changes nothing.
   */
  remember(name: string, value: string, where: Remember, scope?: string): void {
    if (where === 'global') {
      this.setGlobal(name, value);
      if (scope) {
        this.dropScoped(scope, name);
      }
    } else if (where === 'here' && scope) {
      const own = { ...this.scoped(scope), [name]: value };
      write(`personal.scoped.${scope}`, own);
      this.version.update((v) => v + 1);
    }
  }

  setGlobal(name: string, value: string): void {
    const all = { ...this.global(), [name]: value };
    this.global.set(all);
    write('personal.values', all);
  }

  forgetGlobal(name: string): void {
    const all = { ...this.global() };
    delete all[name];
    this.global.set(all);
    write('personal.values', all);
  }

  /** Forgets an item's own values (the item was deleted). */
  clearScope(scope: string): void {
    writePreference(`personal.scoped.${scope}`, '');
    this.version.update((v) => v + 1);
  }

  private dropScoped(scope: string, name: string): void {
    const own = this.scoped(scope);
    if (name in own) {
      delete own[name];
      write(`personal.scoped.${scope}`, own);
      this.version.update((v) => v + 1);
    }
  }
}

function read(key: string): Record<string, string> {
  try {
    const parsed: unknown = JSON.parse(readPreference(key) || '{}');
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return Object.fromEntries(Object.entries(parsed).filter(([, v]) => typeof v === 'string')) as Record<string, string>;
    }
  } catch {
    // A damaged value: start again.
  }
  return {};
}

function write(key: string, values: Record<string, string>): void {
  writePreference(key, JSON.stringify(values));
}
