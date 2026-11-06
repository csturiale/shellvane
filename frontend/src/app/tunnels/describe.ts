import type { TunnelProfile } from '../core/backend/tunnel-api';

/** Short technical summary, the way ssh options read: L/R/D bind → dest. */
export function describeTunnel(p: Pick<TunnelProfile, 'kind' | 'bindAddr' | 'bindPort' | 'destHost' | 'destPort'>, addr = ''): string {
  // A running tunnel shows where it actually listens (port 0 picks one).
  const bind = addr || `${p.bindAddr}:${p.bindPort}`;
  switch (p.kind) {
    case 'local':
      return `L ${bind} → ${dest(p)}`;
    case 'remote':
      return `R ${bind} → ${dest(p)}`;
    default:
      return `D ${bind} (SOCKS5)`;
  }
}

/** host:port, or the path of a Unix socket. */
function dest(p: Pick<TunnelProfile, 'destHost' | 'destPort'>): string {
  return p.destHost.startsWith('/') ? p.destHost : `${p.destHost}:${p.destPort}`;
}
