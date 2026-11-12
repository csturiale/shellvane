/**
 * Snippet variables: {{name}} asks for a value, {{name:a|b|c}} offers choices.
 * System variables are filled from the target pane: host, user, port, date,
 * time. Commands are never translated: values are inserted verbatim.
 */
export interface Variable {
  name: string;
  choices: string[];
}

export const SYSTEM_VARIABLES = ['host', 'user', 'port', 'date', 'time'] as const;

const PATTERN = /\{\{\s*([A-Za-z_][\w-]*)\s*(?::([^}]*))?\}\}/g;

/** Variables the user must fill in, in order of first appearance. */
export function userVariables(body: string): Variable[] {
  const seen = new Map<string, Variable>();
  for (const m of body.matchAll(PATTERN)) {
    const name = m[1];
    if ((SYSTEM_VARIABLES as readonly string[]).includes(name) || seen.has(name)) {
      continue;
    }
    const choices = m[2] ? m[2].split('|').map((c) => c.trim()).filter(Boolean) : [];
    seen.set(name, { name, choices });
  }
  return [...seen.values()];
}

/** Replaces every variable; unknown ones are left as they are. */
export function render(body: string, values: Record<string, string>): string {
  return body.replace(PATTERN, (match, name: string) => (name in values ? values[name] : match));
}

export function systemValues(target: { host: string; user: string; port: number } | null, now = new Date()): Record<string, string> {
  const pad = (n: number) => String(n).padStart(2, '0');
  return {
    host: target?.host ?? '',
    user: target?.user ?? '',
    port: target ? String(target.port) : '',
    date: `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`,
    time: `${pad(now.getHours())}:${pad(now.getMinutes())}:${pad(now.getSeconds())}`,
  };
}

/**
 * Turns a snippet into terminal input: newlines become carriage returns (what
 * the Enter key sends), and Enter is pressed at the end when run is set.
 */
export function toTerminalInput(text: string, run: boolean): string {
  const body = text.replace(/\r?\n/g, '\r').replace(/\r+$/, '');
  return run ? body + '\r' : body;
}

/** Default patterns of commands that ask for confirmation when broadcast. */
export const DEFAULT_DANGEROUS = [
  String.raw`\brm\s+(-\w*\s+)*-\w*[rf]`,
  String.raw`\b(reboot|shutdown|halt|poweroff)\b`,
  String.raw`\bmkfs(\.\w+)?\b`,
  String.raw`\bdd\s+.*\bof=`,
  String.raw`\b(drop|truncate)\s+(table|database)\b`,
  String.raw`:\(\)\s*\{\s*:\|:&\s*\};:`,
];

/** Compiles patterns case-insensitively; invalid ones are skipped. */
export function compileDangerous(patterns: string[]): RegExp[] {
  const out: RegExp[] = [];
  for (const p of patterns) {
    try {
      out.push(new RegExp(p, 'i'));
    } catch {
      // Reported by the settings page.
    }
  }
  return out;
}

const DEFAULT_RES = compileDangerous(DEFAULT_DANGEROUS);

export function isDangerous(command: string, patterns: RegExp[] = DEFAULT_RES): boolean {
  return patterns.some((re) => re.test(command));
}

/** The scope of a snippet's own personal values. */
export function snippetScope(id: string): string {
  return `snippet:${id}`;
}
