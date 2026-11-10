import { Injectable } from '@angular/core';

import { NetworkService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { ReachResult } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { Config as ProxyConfig } from '../../../../bindings/github.com/csturiale/shellvane/core/netproxy';

export type { ProxyConfig, ReachResult };

/** Settings › Network. Tests replace this class with a fake. */
@Injectable({ providedIn: 'root' })
export class NetworkApi {
  proxy(): Promise<ProxyConfig> {
    return NetworkService.Proxy();
  }
  setProxy(c: ProxyConfig): Promise<ProxyConfig> {
    return NetworkService.SetProxy(c);
  }
  async test(): Promise<ReachResult[]> {
    return (await NetworkService.Test()) ?? [];
  }
}
