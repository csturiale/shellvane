import { Injectable } from '@angular/core';

import { ImportService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { ImportSource, ImportSummary } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { Host as ImportedHost, Result, Warning } from '../../../../bindings/github.com/csturiale/shellvane/core/importers';

export type { ImportSource, ImportSummary, ImportedHost, Warning };
export type ImportPreview = Result;

@Injectable({ providedIn: 'root' })
export class ImportApi {
  async sources(): Promise<ImportSource[]> {
    return (await ImportService.Sources()) ?? [];
  }
  preview(source: string, kind: string, path: string): Promise<ImportPreview> {
    return ImportService.Preview(source, kind, path);
  }
  import(hosts: ImportedHost[], group: string): Promise<ImportSummary> {
    return ImportService.Import({ hosts, group });
  }
}
