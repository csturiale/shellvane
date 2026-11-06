/** Local addresses printed by servers ("listening on http://localhost:3000"). */
const LOCAL_URL = /(?:https?:\/\/)?(localhost|127\.0\.0\.1|0\.0\.0\.0|\[::1?\]):(\d{2,5})(?![\d.])/g;

/** A port a program printed, with the address it listens on. */
export interface DetectedPort {
  port: number;
  addr: string;
}

/** Ports of local URLs in terminal output. */
export function detectPorts(text: string): DetectedPort[] {
  const out: DetectedPort[] = [];
  for (const m of text.matchAll(LOCAL_URL)) {
    const port = Number(m[2]);
    if (port > 0 && port <= 65535 && !out.some((p) => p.port === port)) {
      const host = m[1].replace(/[[\]]/g, '');
      out.push({ port, addr: host === 'localhost' ? '127.0.0.1' : host === '' ? '::' : host });
    }
  }
  return out;
}
