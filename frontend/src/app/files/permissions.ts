/** Unix permission bits for the permissions dialog. */
export const WHO = ['owner', 'group', 'others'] as const;
export const WHAT = ['read', 'write', 'execute'] as const;
export const SPECIAL = ['setuid', 'setgid', 'sticky'] as const;

export type Who = (typeof WHO)[number];
export type What = (typeof WHAT)[number];
export type Special = (typeof SPECIAL)[number];

export function bit(who: Who, what: What): number {
  return (4 >> WHAT.indexOf(what)) << (3 * (2 - WHO.indexOf(who)));
}

export function specialBit(s: Special): number {
  return 0o4000 >> SPECIAL.indexOf(s);
}

/** "0755"-style text, always four digits. */
export function toOctal(perm: number): string {
  return (perm & 0o7777).toString(8).padStart(4, '0');
}

/** Parses 3 or 4 octal digits; null when invalid. */
export function parseOctal(text: string): number | null {
  const t = text.trim();
  return /^[0-7]{3,4}$/.test(t) ? parseInt(t, 8) : null;
}

/** "rwxr-x---"-style text, with s/S/t/T for the special bits. */
export function toSymbolic(perm: number): string {
  let out = '';
  for (const who of WHO) {
    for (const what of WHAT) {
      out += perm & bit(who, what) ? what[0].replace('e', 'x') : '-';
    }
  }
  const chars = out.split('');
  const special: [number, number, string][] = [
    [0o4000, 2, 's'],
    [0o2000, 5, 's'],
    [0o1000, 8, 't'],
  ];
  for (const [mask, i, ch] of special) {
    if (perm & mask) {
      chars[i] = chars[i] === '-' ? ch.toUpperCase() : ch;
    }
  }
  return chars.join('');
}
