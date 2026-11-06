import { Injectable, signal } from '@angular/core';

import { readPreference, writePreference } from '../core/preferences';

/** App-wide session behaviour; hosts and groups may turn reconnection off. */
@Injectable({ providedIn: 'root' })
export class SessionPrefs {
  readonly autoReconnect = signal(readPreference('sessions.autoReconnect') !== 'false');
  readonly restoreOnStart = signal(readPreference('sessions.restoreOnStart') !== 'false');
  /** Local terminals: the shell ('' = login shell) and kubectl context. */
  readonly localShell = signal(readPreference('local.shell') ?? '');
  readonly localKubeContext = signal(readPreference('local.kubeContext') ?? '');
  readonly localKubeNamespace = signal(readPreference('local.kubeNamespace') ?? '');
  /** Contexts matching this regex get a red badge. */
  readonly kubeProdPattern = signal(readPreference('kube.prodPattern') ?? 'prod');

  setAutoReconnect(on: boolean): void {
    this.autoReconnect.set(on);
    writePreference('sessions.autoReconnect', String(on));
  }

  setLocal(shell: string, context: string, namespace: string): void {
    this.localShell.set(shell);
    this.localKubeContext.set(context.trim());
    this.localKubeNamespace.set(namespace.trim());
    writePreference('local.shell', shell);
    writePreference('local.kubeContext', context.trim());
    writePreference('local.kubeNamespace', namespace.trim());
  }

  setKubeProdPattern(pattern: string): void {
    this.kubeProdPattern.set(pattern);
    writePreference('kube.prodPattern', pattern);
  }

  /** Whether a context looks like production. */
  isProd(context: string): boolean {
    try {
      return !!context && !!this.kubeProdPattern() && new RegExp(this.kubeProdPattern(), 'i').test(context);
    } catch {
      return false;
    }
  }

  setRestoreOnStart(on: boolean): void {
    this.restoreOnStart.set(on);
    writePreference('sessions.restoreOnStart', String(on));
  }
}
