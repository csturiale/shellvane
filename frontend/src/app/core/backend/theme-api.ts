import { Injectable } from '@angular/core';

import { ThemeService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { Theme as GenTheme } from '../../../../bindings/github.com/csturiale/shellvane/core/themes';

export interface Theme extends Omit<GenTheme, 'colors'> {
  colors: Record<string, string>;
}

function normalize(t: GenTheme): Theme {
  return { ...t, colors: (t.colors ?? {}) as Record<string, string> };
}

/** Custom terminal color schemes. Tests replace this class with a fake. */
@Injectable({ providedIn: 'root' })
export class ThemeApi {
  async list(): Promise<Theme[]> {
    return ((await ThemeService.List()) ?? []).map(normalize);
  }
  async save(t: Theme): Promise<Theme> {
    return normalize(await ThemeService.Save(t));
  }
  delete(id: string): Promise<void> {
    return ThemeService.Delete(id);
  }
  async import(fileName: string, content: string): Promise<Theme[]> {
    return ((await ThemeService.Import(fileName, content)) ?? []).map(normalize);
  }
  export(t: Theme): Promise<string> {
    return ThemeService.Export(t);
  }
}
