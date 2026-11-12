import { Injectable, Injector, Type, inject } from '@angular/core';

import { MenuItem } from '../shared/context-menu.service';
import { CredentialSource, PaletteCommand, ProcedureSourceInput, ShareKind, SharedProcedureSource, SharingProvider } from './extension';
import { EXTENSIONS } from './extensions';

/**
 * What the app asks about sharing. Without an extension that provides it
 * (the community edition) nothing is shared: no menus, badges or reviews.
 */
@Injectable({ providedIn: 'root' })
export class Sharing {
  private readonly ext = EXTENSIONS.find((e) => e.sharing)?.sharing ?? null;
  private readonly provider: SharingProvider | null = this.ext ? inject(Injector).get(this.ext.provider) : null;
  /** Components to render, or null. */
  readonly button: Type<unknown> | null = this.ext?.button ?? null;
  readonly note: Type<unknown> | null = this.ext?.note ?? null;
  readonly review: Type<unknown> | null = this.ext?.review ?? null;
  readonly groupBadge: Type<unknown> | null = this.ext?.groupBadge ?? null;

  start(): void {
    this.provider?.start();
  }

  needsReview(kind: ShareKind | 'group', localId: string): boolean {
    return this.provider?.needsReview(kind, localId) ?? false;
  }

  groupMenu(group: { id: string; name: string }, run: (work: () => Promise<unknown>) => void): (MenuItem | null)[] {
    return this.provider?.groupMenu(group, run) ?? [];
  }

  paletteCommands(): PaletteCommand[] {
    return this.provider?.paletteCommands() ?? [];
  }

  credentialSources(): Promise<CredentialSource[]> {
    return this.provider?.credentialSources() ?? Promise.resolve([]);
  }

  procedureSources(): Promise<SharedProcedureSource[]> {
    return this.provider?.procedureSources() ?? Promise.resolve([]);
  }

  procedureShareMenu(src: ProcedureSourceInput, run: (work: () => Promise<unknown>) => void): (MenuItem | null)[] {
    return this.provider?.procedureShareMenu(src, run) ?? [];
  }
}
