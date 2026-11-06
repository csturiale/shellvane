import { Injectable } from '@angular/core';

import { GalleryService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { GalleryIndex, GalleryResult } from '../../../../bindings/github.com/csturiale/shellvane/app';

export type { GalleryIndex, GalleryResult };

/** The website's community gallery of snippets and color schemes. */
@Injectable({ providedIn: 'root' })
export class GalleryApi {
  async gallery(): Promise<GalleryIndex> {
    const g = await GalleryService.Gallery();
    return { snippets: g.snippets ?? [], themes: g.themes ?? [] };
  }
  install(kind: 'snippets' | 'themes', file: string): Promise<GalleryResult> {
    return GalleryService.Install(kind, file);
  }
}
