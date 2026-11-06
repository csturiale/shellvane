import { Injectable } from '@angular/core';

import { DownloadsService } from '../../../../bindings/github.com/csturiale/shellvane/app';

/** Saves files received in a terminal in the Downloads folder, in chunks. */
@Injectable({ providedIn: 'root' })
export class DownloadsApi {
  begin(name: string): Promise<string> {
    return DownloadsService.Begin(name);
  }
  append(id: string, data: Uint8Array): Promise<void> {
    return DownloadsService.Append(id, toBase64(data));
  }
  finish(id: string): Promise<string> {
    return DownloadsService.Finish(id);
  }
  cancel(id: string): Promise<void> {
    return DownloadsService.Cancel(id);
  }
}

/** Base64 of bytes; a binding carries []byte as base64. */
export function toBase64(data: Uint8Array): string {
  let binary = '';
  const step = 0x8000;
  for (let i = 0; i < data.length; i += step) {
    binary += String.fromCharCode(...data.subarray(i, i + step));
  }
  return btoa(binary);
}
