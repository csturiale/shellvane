import { Injectable, inject, signal } from '@angular/core';

import { TranslatableError, toTranslatableError } from '../core/backend/errors';
import { UpdateApi, UpdateInfo } from '../core/backend/update-api';
import { readPreference, writePreference } from '../core/preferences';

const DAY = 24 * 60 * 60 * 1000;

/** Update checks: on demand, and once a day when allowed. */
@Injectable({ providedIn: 'root' })
export class UpdatesStore {
  private readonly api = inject(UpdateApi);
  readonly info = signal<UpdateInfo | null>(null);
  readonly checking = signal(false);
  readonly installing = signal(false);
  readonly installed = signal(false);
  readonly error = signal<TranslatableError | null>(null);
  readonly auto = signal(readPreference('updates.auto') !== 'false');

  setAuto(on: boolean): void {
    this.auto.set(on);
    writePreference('updates.auto', String(on));
  }

  /** At startup: checks when allowed and the last check is a day old. */
  async checkDaily(): Promise<void> {
    const last = Number(readPreference('updates.lastCheck') ?? 0);
    if (this.auto() && Date.now() - last > DAY) {
      await this.check().catch(() => undefined);
    }
  }

  async check(): Promise<void> {
    this.checking.set(true);
    this.error.set(null);
    try {
      this.info.set(await this.api.check());
      writePreference('updates.lastCheck', String(Date.now()));
    } catch (err) {
      this.error.set(toTranslatableError(err));
    } finally {
      this.checking.set(false);
    }
  }

  async install(): Promise<void> {
    this.installing.set(true);
    this.error.set(null);
    try {
      await this.api.install();
      this.installed.set(true);
    } catch (err) {
      this.error.set(toTranslatableError(err));
    } finally {
      this.installing.set(false);
    }
  }

  restart(): Promise<void> {
    return this.api.restart();
  }
}
