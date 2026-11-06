/** Formats a byte count for the given language, e.g. "1.2 MB" / "1,2 MB". */
export function formatBytes(bytes: number, lang: string): string {
  const units = ['byte', 'kilobyte', 'megabyte', 'gigabyte', 'terabyte'];
  let value = bytes;
  let i = 0;
  while (value >= 1000 && i < units.length - 1) {
    value /= 1000;
    i++;
  }
  return new Intl.NumberFormat(lang, {
    style: 'unit',
    unit: units[i],
    unitDisplay: 'short',
    maximumFractionDigits: i === 0 ? 0 : 1,
  }).format(value);
}
