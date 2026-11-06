import { Injectable } from '@angular/core';

import { DiagnosticsService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type * as gen from '../../../../bindings/github.com/csturiale/shellvane/core/diagnostics';
import type { File as BundleFile } from '../../../../bindings/github.com/csturiale/shellvane/core/diagnostics';

export type { BundleFile };

export interface TraceEvent {
  ms: number;
  step: string;
  detail: Record<string, string>;
}

export interface Handshake extends Omit<gen.Handshake, 'events'> {
  events: TraceEvent[];
}

/** Connection diagnostics and the support bundle. Tests replace this class. */
@Injectable({ providedIn: 'root' })
export class DiagnosticsApi {
  async handshakes(): Promise<Handshake[]> {
    return ((await DiagnosticsService.Handshakes()) ?? []).map((h) => ({
      ...h,
      events: (h.events ?? []).map((e) => ({ ms: e.ms, step: e.step, detail: (e.detail ?? {}) as Record<string, string> })),
    }));
  }
  report(id: string): Promise<string> {
    return DiagnosticsService.HandshakeReport(id);
  }
  async preview(preferences: Record<string, string>): Promise<BundleFile[]> {
    return (await DiagnosticsService.BundlePreview(preferences)) ?? [];
  }
  save(preferences: Record<string, string>): Promise<string> {
    return DiagnosticsService.SaveBundle(preferences);
  }
  reveal(path: string): Promise<void> {
    return DiagnosticsService.RevealBundle(path);
  }
}
