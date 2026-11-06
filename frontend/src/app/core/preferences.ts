// Per-device UI preferences. These move to the Go settings store once it
// exists; until then localStorage is enough.

export function readPreference(key: string): string | null {
  try {
    return localStorage.getItem(`shellvane.${key}`);
  } catch {
    return null;
  }
}

export function writePreference(key: string, value: string): void {
  try {
    localStorage.setItem(`shellvane.${key}`, value);
  } catch {
    // Storage unavailable: the preference lasts for this session only.
  }
}

/** Every preference, keys without the prefix (for the support bundle). */
export function allPreferences(): Record<string, string> {
  const out: Record<string, string> = {};
  try {
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i);
      if (key?.startsWith('shellvane.')) {
        out[key.slice('shellvane.'.length)] = localStorage.getItem(key) ?? '';
      }
    }
  } catch {
    // Storage unavailable.
  }
  return out;
}
