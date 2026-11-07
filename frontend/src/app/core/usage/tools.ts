/**
 * Well-known command-line programs the opt-in statistics may count, and how
 * a command line is reduced to one of them. Loaded only after consent.
 */
export const TOOLS: ReadonlySet<string> = new Set([
  // Containers and orchestration
  'docker', 'docker-compose', 'podman', 'kubectl', 'k9s', 'helm', 'kubectx', 'kubens', 'oc', 'minikube', 'kind', 'k3s', 'crictl', 'nerdctl', 'stern', 'kustomize', 'argocd', 'flux',
  // Clouds and infrastructure as code
  'aws', 'az', 'gcloud', 'gsutil', 'doctl', 'hcloud', 'oci', 'terraform', 'tofu', 'terragrunt', 'pulumi', 'ansible', 'ansible-playbook', 'packer', 'vagrant', 'vault', 'consul', 'nomad', 'tsh', 'boundary',
  // Services and logs
  'systemctl', 'journalctl', 'service', 'supervisorctl', 'pm2', 'nginx', 'apachectl', 'caddy', 'tail', 'less', 'dmesg', 'logrotate',
  // Diagnostics
  'top', 'htop', 'btop', 'ps', 'free', 'df', 'du', 'iostat', 'vmstat', 'lsof', 'strace', 'ss', 'netstat', 'ip', 'ifconfig', 'ping', 'traceroute', 'mtr', 'dig', 'nslookup', 'curl', 'wget', 'nc', 'tcpdump', 'nmap', 'openssl', 'uptime',
  // Databases
  'psql', 'mysql', 'mariadb', 'redis-cli', 'mongosh', 'mongo', 'sqlite3', 'pg_dump', 'mysqldump',
  // Files and text
  'ls', 'cd', 'cat', 'grep', 'rg', 'find', 'fd', 'awk', 'sed', 'jq', 'yq', 'vim', 'vi', 'nvim', 'nano', 'emacs', 'cp', 'mv', 'rm', 'mkdir', 'chmod', 'chown', 'tar', 'zip', 'unzip', 'rsync', 'scp', 'sftp', 'ssh', 'tmux', 'screen', 'watch',
  // Packages and languages
  'apt', 'apt-get', 'dnf', 'yum', 'zypper', 'apk', 'pacman', 'brew', 'snap', 'pip', 'pip3', 'npm', 'npx', 'pnpm', 'yarn', 'node', 'python', 'python3', 'go', 'cargo', 'java', 'mvn', 'gradle', 'dotnet', 'php', 'composer', 'ruby', 'gem', 'make',
  // Version control
  'git', 'gh', 'glab', 'svn',
]);

/** Wrappers looked through to find the program a command runs. */
const WRAPPERS = new Set(['sudo', 'doas', 'time', 'nohup', 'exec', 'env', 'nice', 'command', 'builtin', 'watch']);
/** Wrapper options followed by a value (sudo -u user, nice -n 10…). */
const VALUED = new Set(['-u', '-g', '-C', '-D', '-h', '-p', '-U', '-n', '-r', '-t', '-T']);

/**
 * The well-known program a command line runs, or null: leading variable
 * assignments and wrappers (sudo, env, time…) are skipped, a path keeps its
 * last part. Anything not in TOOLS gives null.
 */
export function toolOf(command: string): string | null {
  const words = command.trim().split(/\s+/);
  let i = 0;
  let wrapped = false;
  while (i < words.length) {
    const w = words[i];
    if (/^[A-Za-z_][A-Za-z0-9_]*=/.test(w)) {
      i++;
      continue;
    }
    if (w.startsWith('-') && i > 0 && wrapped) {
      i += VALUED.has(w) ? 2 : 1;
      continue;
    }
    if (WRAPPERS.has(w) && i + 1 < words.length) {
      wrapped = true;
      i++;
      continue;
    }
    break;
  }
  const word = (words[i] ?? '').replace(/^.*\//, '').toLowerCase();
  return TOOLS.has(word) ? word : null;
}
