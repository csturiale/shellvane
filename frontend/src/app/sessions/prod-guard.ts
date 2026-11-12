/**
 * Commands that change a Kubernetes cluster in ways hard to undo: asked
 * for confirmation before Enter reaches the shell when the pane, or the
 * command's own --context, is a production context.
 */
const RISKY: RegExp[] = [
  /\bkubectl\b.*\s(delete|drain|cordon|taint|replace)\b/,
  /\bkubectl\b.*\sscale\b.*--replicas[= ]0\b/,
  /\bkubectl\b.*\srollout\s+(restart|undo)\b/,
  /\bkubectl\b.*\sapply\b.*--prune\b/,
  /\bkubectl\b.*\s(edit|patch|set|label|annotate)\b/,
  /\bhelm\b.*\s(uninstall|delete|rollback|upgrade|install)\b/,
];

/** Whether cmd is one of the risky kubectl or helm commands. */
export function isRiskyKubeCommand(cmd: string): boolean {
  const c = ' ' + cmd.replace(/\s+/g, ' ').trim();
  return RISKY.some((re) => re.test(c));
}

/** The context named in cmd with --context (or --kube-context for helm), "" when none. */
export function contextInCommand(cmd: string): string {
  const m = /--(?:kube-)?context[= ]+["']?([^\s"']+)/.exec(cmd);
  return m?.[1] ?? '';
}
