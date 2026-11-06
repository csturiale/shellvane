import { ImportedHost } from '../core/backend/import-api';

/** A row of the import preview: a group (folder) or a host. */
export type ImportRow =
  | { kind: 'group'; key: string; name: string; depth: number; hosts: number[] }
  | { kind: 'host'; index: number; depth: number };

/**
 * Builds the preview tree from the hosts' group paths: groups first, by
 * name, then the hosts of each group in their original order. A group row
 * lists the indexes of every host below it (subgroups included), for its
 * checkbox. Rows inside a collapsed group are left out.
 */
export function importRows(hosts: ImportedHost[], collapsed: ReadonlySet<string> = new Set()): ImportRow[] {
  interface Node {
    name: string;
    key: string;
    children: Map<string, Node>;
    hosts: number[];
  }
  const root: Node = { name: '', key: '', children: new Map(), hosts: [] };
  hosts.forEach((h, i) => {
    let node = root;
    for (const part of h.group ?? []) {
      const name = part.trim();
      if (!name) {
        continue;
      }
      let child = node.children.get(name.toLowerCase());
      if (!child) {
        child = { name, key: node.key + '/' + name.toLowerCase(), children: new Map(), hosts: [] };
        node.children.set(name.toLowerCase(), child);
      }
      node = child;
    }
    node.hosts.push(i);
  });
  const all = (n: Node): number[] => [...n.hosts, ...[...n.children.values()].flatMap(all)];
  const rows: ImportRow[] = [];
  const walk = (n: Node, depth: number) => {
    const children = [...n.children.values()].sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }));
    for (const c of children) {
      rows.push({ kind: 'group', key: c.key, name: c.name, depth, hosts: all(c) });
      if (!collapsed.has(c.key)) {
        walk(c, depth + 1);
      }
    }
    for (const i of n.hosts) {
      rows.push({ kind: 'host', index: i, depth });
    }
  };
  walk(root, 0);
  return rows;
}

/** The hosts to import, with the default user where a host has none. */
export function withDefaultUser(hosts: ImportedHost[], user: string): ImportedHost[] {
  const u = user.trim();
  return u ? hosts.map((h) => (h.user ? h : { ...h, user: u })) : hosts;
}
