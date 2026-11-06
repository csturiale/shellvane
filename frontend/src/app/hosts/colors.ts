/** Group colors are stored as names so each theme can choose its own shade. */
export const GROUP_COLORS = ['gray', 'red', 'orange', 'yellow', 'green', 'teal', 'blue', 'purple', 'pink'] as const;
export type GroupColor = (typeof GROUP_COLORS)[number];

export function colorVar(color: string): string {
  return GROUP_COLORS.includes(color as GroupColor) ? `var(--group-${color})` : 'var(--text-muted)';
}
