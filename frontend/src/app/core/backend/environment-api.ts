import { Injectable } from '@angular/core';

import { EnvironmentService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { LoginCheck } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { Environment } from '../../../../bindings/github.com/csturiale/shellvane/core/environments';

export type { Environment, LoginCheck };

/** Environments. Tests replace this class with a fake. */
@Injectable({ providedIn: 'root' })
export class EnvironmentApi {
  async list(): Promise<Environment[]> {
    return (await EnvironmentService.List()) ?? [];
  }
  save(e: Environment): Promise<Environment> {
    return EnvironmentService.Save(e);
  }
  delete(id: string): Promise<void> {
    return EnvironmentService.Delete(id);
  }
  checkLogin(command: string, values: Record<string, string>): Promise<LoginCheck> {
    return EnvironmentService.CheckLogin(command, values);
  }
  export(id: string): Promise<string> {
    return EnvironmentService.Export(id);
  }
  import(text: string): Promise<Environment> {
    return EnvironmentService.Import(text);
  }
}
