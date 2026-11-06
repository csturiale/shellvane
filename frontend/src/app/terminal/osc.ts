/** A notification asked by a program with OSC 9 or OSC 777. */
export interface OscNotification {
  title: string;
  body: string;
}

/**
 * OSC 9 (iTerm2, Windows Terminal): "message". "4;..." is a progress
 * report (ConEmu), not a notification.
 */
export function parseOsc9(data: string): OscNotification | null {
  if (!data || /^4;/.test(data)) {
    return null;
  }
  return { title: '', body: data };
}

/** OSC 777 (rxvt, Ghostty, foot): "notify;title;body". */
export function parseOsc777(data: string): OscNotification | null {
  const [kind, title = '', ...rest] = data.split(';');
  if (kind !== 'notify') {
    return null;
  }
  return { title, body: rest.join(';') };
}

/**
 * Shell integration (OSC 133, FinalTerm): marks prompts and commands.
 * C starts the output of a command, D;exit ends it.
 */
export class CommandTracker {
  private startedAt = 0;

  /** Returns the finished command's duration (ms) and exit code on D. */
  feed(data: string, now = Date.now()): { ms: number; exit: number | null } | null {
    const [mark, code] = data.split(';');
    if (mark === 'C') {
      this.startedAt = now;
      return null;
    }
    if (mark === 'D' && this.startedAt > 0) {
      const ms = now - this.startedAt;
      this.startedAt = 0;
      const exit = code === undefined || code === '' ? null : Number(code);
      return { ms, exit: Number.isFinite(exit) ? exit : null };
    }
    if (mark === 'A') {
      this.startedAt = 0; // a new prompt: no command running
    }
    return null;
  }
}
