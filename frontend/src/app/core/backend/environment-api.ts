import { Injectable } from '@angular/core';

import { EnvironmentService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { CopiedSecret, LoginCheck } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { Environment } from '../../../../bindings/github.com/csturiale/shellvane/core/environments';

export type { CopiedSecret, Environment, LoginCheck };

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
  checkLogin(command: string, values: Record<string, string>, gcloud = ''): Promise<LoginCheck> {
    return EnvironmentService.CheckLogin(command, values, gcloud);
  }
  /** Copies the login secret's password to the clipboard (it never reaches the UI). */
  /** cancel() stops it (a sign-in waiting in the browser, the user skips the password). */
  copyLoginSecret(envId: string, values: Record<string, string>, seconds = 0): Promise<CopiedSecret> & { cancel?: () => void } {
    return EnvironmentService.CopyLoginSecret(envId, values, seconds);
  }
  async gcloudProfiles(): Promise<string[]> {
    return (await EnvironmentService.GcloudProfiles()) ?? [];
  }
  export(id: string): Promise<string> {
    return EnvironmentService.Export(id);
  }
  import(text: string): Promise<Environment> {
    return EnvironmentService.Import(text);
  }
}
