import { Injectable } from '@angular/core';

import { DialogService } from '../../../../bindings/github.com/csturiale/shellvane/app';

/** The system's file and folder choosers. Tests replace this class with a fake. */
@Injectable({ providedIn: 'root' })
export class DialogApi {
  /** A file, opening near current; "" when cancelled. */
  chooseFile(title: string, current = ''): Promise<string> {
    return DialogService.ChooseFile(title, current);
  }
  /** A folder, opening at current; "" when cancelled. */
  chooseFolder(title: string, current = ''): Promise<string> {
    return DialogService.ChooseFolder(title, current);
  }
}
