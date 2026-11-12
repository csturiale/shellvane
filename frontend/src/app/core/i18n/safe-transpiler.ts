import { Injectable } from '@angular/core';
import { TranslocoTranspiler } from '@jsverse/transloco';
import { MessageFormatTranspiler } from '@jsverse/transloco-messageformat';

/** Characters with a meaning in ICU MessageFormat. */
const ICU_SPECIAL = /[{}'#]/;

/**
 * The MessageFormat transpiler, safe for text from the user: params are
 * put into the message before ICU compiles it, so a command such as
 * `kubectl -n "${NS}"` came out as `kubectl -n "$undefined"`, and a lone
 * brace could break the message. String params that hold ICU characters
 * go in as plain tokens and are put back after compiling.
 */
@Injectable()
export class SafeMessageFormatTranspiler extends MessageFormatTranspiler implements TranslocoTranspiler {
  override transpile(args: Parameters<MessageFormatTranspiler['transpile']>[0]): ReturnType<MessageFormatTranspiler['transpile']> {
    const params = (args.params ?? {}) as Record<string, unknown>;
    const kept: string[] = [];
    let safe: Record<string, unknown> | null = null;
    for (const [k, v] of Object.entries(params)) {
      if (typeof v === 'string' && ICU_SPECIAL.test(v)) {
        safe ??= { ...params };
        safe[k] = `${kept.length}`;
        kept.push(v);
      }
    }
    if (!safe) {
      return super.transpile(args);
    }
    const out = super.transpile({ ...args, params: safe });
    return typeof out === 'string' ? out.replace(/(\d+)/g, (_, i: string) => kept[+i]) : out;
  }
}
