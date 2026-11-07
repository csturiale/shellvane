import { Injectable, computed, inject, signal } from '@angular/core';

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
  /** A notice offers the update when one is found (off: only Settings and the status bar). */
  readonly notify = signal(readPreference('updates.notify') !== 'false');
  /** A version the user chose to skip. */
  private readonly skipped = signal(readPreference('updates.skip') ?? '');
  /** Until when "Remind me tomorrow" keeps the notice away. */
  private readonly snoozedUntil = signal(Number(readPreference('updates.snoozeUntil') ?? 0));
  /** Closed for this run of the app. */
  private readonly dismissed = signal(false);

  /** The notice is shown: an update to offer, not skipped, snoozed or turned off. */
  readonly prompt = computed(() => {
    const i = this.info();
    if (this.installed()) {
      return !this.dismissed(); // installed: offer the restart
    }
    return (
      !!i?.enabled &&
      i.available &&
      this.notify() &&
      !this.dismissed() &&
      i.latest !== this.skipped() &&
      Date.now() >= this.snoozedUntil()
    );
  });

  setNotify(on: boolean): void {
    this.notify.set(on);
    writePreference('updates.notify', String(on));
  }

  /** "Remind me tomorrow". */
  snooze(): void {
    const until = Date.now() + DAY;
    this.snoozedUntil.set(until);
    writePreference('updates.snoozeUntil', String(until));
  }

  /** "Skip this version": the next one is offered again. */
  skip(): void {
    const v = this.info()?.latest ?? '';
    this.skipped.set(v);
    writePreference('updates.skip', v);
  }

  /** Closes the notice until the next start. */
  dismiss(): void {
    this.dismissed.set(true);
  }

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
