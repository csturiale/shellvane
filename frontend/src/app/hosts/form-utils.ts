import { Group, Resolved } from '../core/backend/host-api';

/** Parses an optional port field: empty means "inherit" (null). */
export function parsePort(value: string | number | null | undefined): number | null {
  if (value === null || value === undefined || String(value).trim() === '') {
    return null;
  }
  return Number(value);
}

/** Parses an optional text field: empty means "inherit" (null). */
export function optionalText(value: string | null | undefined): string | null {
  const v = (value ?? '').trim();
  return v === '' ? null : v;
}

export interface InheritedValue {
  value: string;
  /** Name of the group that set the value, or null for built-in defaults. */
  from: string | null;
}

/** Describes where an inherited setting comes from, for placeholders. */
export function inheritedValue(resolved: Resolved | null, field: 'user' | 'port', groups: Group[]): InheritedValue | null {
  const raw = resolved?.settings[field];
  if (raw === null || raw === undefined) {
    return null;
  }
  const sourceId = resolved?.source?.[field] ?? '';
  return { value: String(raw), from: groups.find((g) => g.id === sourceId)?.name ?? null };
}

/** One command per line; no lines means "inherit" (null). */
export function parseLines(text: string | null | undefined): string[] | null {
  const lines = (text ?? '')
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean);
  return lines.length ? lines : null;
}

/** Inherited startup commands and the group that set them, for placeholders. */
export function inheritedCommands(resolved: Resolved | null, groups: Group[]): InheritedValue | null {
  const cmds = resolved?.settings.startupCommands;
  if (!cmds?.length) {
    return null;
  }
  const sourceId = resolved?.source?.['startupCommands'] ?? '';
  return { value: cmds.join('\n'), from: groups.find((g) => g.id === sourceId)?.name ?? null };
}
