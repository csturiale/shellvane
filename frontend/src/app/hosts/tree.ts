import type { Group, Host } from '../core/backend/host-api';

export type TreeRow =
  | { kind: 'group'; group: Group; depth: number; expanded: boolean; hostCount: number; hasChildren: boolean }
  | { kind: 'host'; host: Host; depth: number };

/**
 * Flattens the inventory into visible rows: groups first, then hosts, each
 * level sorted by name. With a filter, matching hosts are shown with all their
 * ancestor groups expanded, and groups whose name matches show their contents.
 */
export function buildRows(groups: Group[], hosts: Host[], expanded: ReadonlySet<string>, filter = ''): TreeRow[] {
  const query = filter.trim().toLowerCase();
  const childGroups = new Map<string, Group[]>();
  const childHosts = new Map<string, Host[]>();
  for (const g of groups) {
    push(childGroups, g.parentId, g);
  }
  for (const h of hosts) {
    push(childHosts, h.groupId, h);
  }
  const byName = (a: { name: string }, b: { name: string }) =>
    a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
  childGroups.forEach((list) => list.sort(byName));
  childHosts.forEach((list) => list.sort(byName));

  const totalHosts = new Map<string, number>();
  const countHosts = (id: string): number => {
    let n = childHosts.get(id)?.length ?? 0;
    for (const g of childGroups.get(id) ?? []) {
      n += countHosts(g.id);
    }
    totalHosts.set(id, n);
    return n;
  };
  countHosts('');

  // "#tag" matches tags only; other text matches name, address, tags and notes.
  const tagQuery = query.startsWith('#') ? query.slice(1) : null;
  const hostMatches = (h: Host) =>
    tagQuery !== null
      ? (h.tags ?? []).some((t) => t.toLowerCase().startsWith(tagQuery))
      : h.name.toLowerCase().includes(query) ||
        h.hostname.toLowerCase().includes(query) ||
        (h.tags ?? []).some((t) => t.toLowerCase().includes(query)) ||
        h.notes.toLowerCase().includes(query);

  // Returns the rows of a subtree, or null when nothing in it matches.
  const walk = (parentId: string, depth: number, forced: boolean): TreeRow[] | null => {
    const rows: TreeRow[] = [];
    for (const g of childGroups.get(parentId) ?? []) {
      const selfMatch = !query || forced || (tagQuery === null && g.name.toLowerCase().includes(query));
      const children = walk(g.id, depth + 1, query !== '' && selfMatch);
      if (!selfMatch && children === null) {
        continue;
      }
      const isOpen = query ? true : expanded.has(g.id);
      rows.push({
        kind: 'group',
        group: g,
        depth,
        expanded: isOpen,
        hostCount: totalHosts.get(g.id) ?? 0,
        hasChildren: (childGroups.get(g.id)?.length ?? 0) + (childHosts.get(g.id)?.length ?? 0) > 0,
      });
      if (isOpen && children) {
        rows.push(...children);
      }
    }
    for (const h of childHosts.get(parentId) ?? []) {
      if (!query || forced || hostMatches(h)) {
        rows.push({ kind: 'host', host: h, depth });
      }
    }
    return rows.length || !query ? rows : null;
  };

  return walk('', 0, false) ?? [];
}

/** IDs of a group and all its descendants: invalid parents when moving it. */
export function subtreeIds(groups: Group[], rootId: string): Set<string> {
  const ids = new Set([rootId]);
  let grew = true;
  while (grew) {
    grew = false;
    for (const g of groups) {
      if (ids.has(g.parentId) && !ids.has(g.id)) {
        ids.add(g.id);
        grew = true;
      }
    }
  }
  return ids;
}

/** Groups as "Parent / Child" paths, sorted, for parent pickers. */
export function groupPaths(groups: Group[]): { id: string; path: string }[] {
  const byId = new Map(groups.map((g) => [g.id, g]));
  const pathOf = (g: Group): string => {
    const parent = byId.get(g.parentId);
    return parent ? `${pathOf(parent)} / ${g.name}` : g.name;
  };
  return groups
    .map((g) => ({ id: g.id, path: pathOf(g) }))
    .sort((a, b) => a.path.localeCompare(b.path, undefined, { sensitivity: 'base' }));
}

function push<T>(map: Map<string, T[]>, key: string, value: T): void {
  const list = map.get(key);
  if (list) {
    list.push(value);
  } else {
    map.set(key, [value]);
  }
}
