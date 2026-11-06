/** An asciinema v2 recording. */
export interface Cast {
  width: number;
  height: number;
  title: string;
  events: CastEvent[];
  /** Seconds, after idle compression. */
  duration: number;
}

export interface CastEvent {
  time: number;
  kind: 'o' | 'r';
  data: string;
}

/**
 * Parses asciinema v2 (a JSON header line, then [time, kind, data] lines).
 * Pauses longer than idleLimit seconds are shortened, as asciinema does.
 */
export function parseCast(text: string, idleLimit = 2): Cast {
  const lines = text.split('\n').filter((l) => l.trim());
  const header = JSON.parse(lines[0] ?? '{}') as { version?: number; width?: number; height?: number; title?: string };
  if (header.version !== 2) {
    throw new Error('not an asciinema v2 recording');
  }
  const events: CastEvent[] = [];
  let last = 0;
  let shift = 0;
  for (const line of lines.slice(1)) {
    let ev: unknown;
    try {
      ev = JSON.parse(line);
    } catch {
      continue; // a truncated last line
    }
    if (!Array.isArray(ev) || typeof ev[0] !== 'number' || (ev[1] !== 'o' && ev[1] !== 'r')) {
      continue;
    }
    const gap = ev[0] - last;
    if (gap > idleLimit) {
      shift += gap - idleLimit;
    }
    last = ev[0];
    events.push({ time: ev[0] - shift, kind: ev[1], data: String(ev[2]) });
  }
  return {
    width: header.width ?? 80,
    height: header.height ?? 24,
    title: header.title ?? '',
    events,
    duration: events.at(-1)?.time ?? 0,
  };
}

/** Index of the first event after time t. */
export function eventsUntil(events: CastEvent[], t: number): number {
  let i = 0;
  while (i < events.length && events[i].time <= t) {
    i++;
  }
  return i;
}

/** "1:05" from seconds. */
export function clock(seconds: number): string {
  const s = Math.max(0, Math.floor(seconds));
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}
