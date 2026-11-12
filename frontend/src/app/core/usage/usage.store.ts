import { Injectable, Injector, inject, signal } from '@angular/core';

import type { UsageApi, UsagePayload } from '../backend/usage-api';
import { readPreference, writePreference } from '../preferences';
import { Feature, VIEWS } from './usage';

const WEEK = 7 * 24 * 60 * 60 * 1000;

interface Counters {
  since: number;
  features: Record<string, number>;
  tools: Record<string, number>;
}

/**
 * Anonymous usage statistics, only after the person agreed (Settings ›
 * Privacy, or the question asked once). Counters stay on this computer and
 * a summary goes to the project once a week; nothing is counted otherwise.
 */
@Injectable({ providedIn: 'root' })
export class UsageStore {
  private readonly injector = inject(Injector);
  /** The backend call is loaded only when a report is shown or sent. */
  private async api(): Promise<UsageApi> {
    const m = await import('../backend/usage-api');
    return this.injector.get(m.UsageApi);
  }
  /** 'yes', 'no', or '' while the person has not answered. */
  readonly consent = signal(readPreference('usage.consent') ?? '');
  readonly lastSent = signal(Number(readPreference('usage.lastSent') ?? 0));

  setConsent(on: boolean): void {
    this.consent.set(on ? 'yes' : 'no');
    writePreference('usage.consent', on ? 'yes' : 'no');
    if (!on) {
      writePreference('usage.counters', ''); // forget what was counted
    } else if (!this.read().since) {
      this.write({ since: Date.now(), features: {}, tools: {} });
    }
  }

  /** One use of a feature. */
  count(feature: Feature): void {
    this.add('features', feature);
  }

  /** A view opened. */
  view(name: string): void {
    if (VIEWS.includes(name)) {
      this.add('features', `view.${name}`);
    }
  }

  /**
   * A command run in a terminal: only a well-known program's name counts.
   * Resolves once it is counted (callers need not wait).
   */
  command(line: string): Promise<void> {
    if (this.consent() !== 'yes') {
      return Promise.resolve();
    }
    // The list of tools is loaded only once the person agreed.
    return import('./tools').then(({ toolOf }) => {
      const tool = toolOf(line);
      if (tool) {
        this.add('tools', tool);
      }
    });
  }

  /** Exactly what the next report sends. */
  preview(): Promise<UsagePayload> {
    return this.api().then((api) => api.payload(this.report()));
  }

  /** At start and every hour: sends the week's report once it is due. */
  async sendIfDue(): Promise<void> {
    const c = this.read();
    if (this.consent() !== 'yes' || !c.since || Date.now() - c.since < WEEK) {
      return;
    }
    if (Object.keys(c.features).length + Object.keys(c.tools).length > 0) {
      await (await this.api()).send(this.report());
    }
    this.write({ since: Date.now(), features: {}, tools: {} });
    this.lastSent.set(Date.now());
    writePreference('usage.lastSent', String(Date.now()));
  }

  private report() {
    const c = this.read();
    const days = c.since ? Math.max(1, Math.round((Date.now() - c.since) / (24 * 60 * 60 * 1000))) : 0;
    return { days, features: c.features, tools: c.tools };
  }

  private add(kind: 'features' | 'tools', key: string): void {
    if (this.consent() !== 'yes') {
      return;
    }
    const c = this.read();
    c.since ||= Date.now();
    c[kind][key] = (c[kind][key] ?? 0) + 1;
    this.write(c);
  }

  private read(): Counters {
    try {
      const c = JSON.parse(readPreference('usage.counters') || '{}') as Partial<Counters>;
      return { since: c.since ?? 0, features: c.features ?? {}, tools: c.tools ?? {} };
    } catch {
      return { since: 0, features: {}, tools: {} };
    }
  }

  private write(c: Counters): void {
    writePreference('usage.counters', JSON.stringify(c));
  }
}
