/** A serial:// spec for a port at the most common console speed. */
export function serialSpec(port: string): string {
  return `serial://${port}?baud=115200`;
}
