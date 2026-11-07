/**
 * What the opt-in usage statistics may count. Features are fixed names of
 * Shellvane's own functions; tools are well-known command-line programs:
 * a command counts only when its program is in this public list, and only
 * the program's name is kept (never arguments, hosts or paths).
 */
export const FEATURES = [
  'session.ssh',
  'session.local',
  'session.quick',
  'session.mosh',
  'session.many',
  'pane.split',
  'broadcast',
  'files',
  'tunnel.start',
  'environment.start',
  'runbook.run',
  'snippet.run',
  'suggestion.accept',
  'palette',
  'gallery.install',
  'plugin.install',
  'theme.import',
  'import',
  'sync',
  'team.share',
  'workspace.open',
  'recording',
  'assistant',
] as const;
export type Feature = (typeof FEATURES)[number];

/** Views opened, counted as `view.<name>`. */
export const VIEWS = ['hosts', 'environments', 'snippets', 'tunnels', 'files', 'keys', 'automation', 'plugins', 'settings', 'guide', 'recordings', 'team'];
