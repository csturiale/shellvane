/**
 * Split layout of a tab: a binary tree whose leaves are panes. Rendering does
 * not follow the tree: every pane is positioned absolutely from the rectangle
 * computed here, so splitting never re-creates (and never clears) a terminal.
 */
export type SplitDir = 'row' | 'column'; // row: side by side, column: stacked

export type Layout =
  | { kind: 'pane'; pane: number }
  | { kind: 'split'; dir: SplitDir; ratio: number; a: Layout; b: Layout };

/** Rectangle in percent of the tab area. */
export interface Rect {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface Divider {
  /** Path of the split in the tree: 'a'/'b' steps from the root. */
  path: string;
  dir: SplitDir;
  /** Area the split divides, needed to turn pointer moves into ratios. */
  area: Rect;
  rect: Rect;
}

export const MIN_RATIO = 0.1;
export const MAX_RATIO = 0.9;

export function leaf(pane: number): Layout {
  return { kind: 'pane', pane };
}

/** Copies a layout with every pane key changed by fn. */
export function mapPanes(node: Layout, fn: (pane: number) => number): Layout {
  return node.kind === 'pane' ? leaf(fn(node.pane)) : { ...node, a: mapPanes(node.a, fn), b: mapPanes(node.b, fn) };
}

export function panesOf(node: Layout): number[] {
  return node.kind === 'pane' ? [node.pane] : [...panesOf(node.a), ...panesOf(node.b)];
}

/**
 * Splits `target` in two: by default the existing pane first and `added`
 * after it (right or below); with `before`, `added` goes left or above.
 */
export function split(node: Layout, target: number, added: number, dir: SplitDir, before = false): Layout {
  if (node.kind === 'pane') {
    if (node.pane !== target) {
      return node;
    }
    const [a, b] = before ? [leaf(added), node] : [node, leaf(added)];
    return { kind: 'split', dir, ratio: 0.5, a, b };
  }
  return { ...node, a: split(node.a, target, added, dir, before), b: split(node.b, target, added, dir, before) };
}

/**
 * Arranges panes in a grid: columns side by side, each split in rows, as
 * square as possible (4 → 2×2, 6 → 3×2).
 */
export function grid(keys: number[]): Layout {
  if (keys.length === 0) {
    throw new Error('grid of no panes');
  }
  const cols = Math.ceil(Math.sqrt(keys.length));
  const columns: number[][] = [];
  const perColumn = Math.ceil(keys.length / cols);
  for (let i = 0; i < keys.length; i += perColumn) {
    columns.push(keys.slice(i, i + perColumn));
  }
  return chain(columns.map((c) => chain(c.map(leaf), 'column')), 'row');
}

/** Splits evenly: [a, b, c] → a | (b | c) with ratios 1/3 and 1/2. */
function chain(nodes: Layout[], dir: SplitDir): Layout {
  if (nodes.length === 1) {
    return nodes[0];
  }
  return { kind: 'split', dir, ratio: 1 / nodes.length, a: nodes[0], b: chain(nodes.slice(1), dir) };
}

/** Exchanges two panes of the same layout. */
export function swap(node: Layout, x: number, y: number): Layout {
  if (node.kind === 'pane') {
    return node.pane === x ? leaf(y) : node.pane === y ? leaf(x) : node;
  }
  return { ...node, a: swap(node.a, x, y), b: swap(node.b, x, y) };
}

/** Replaces one pane with another (used when swapping across tabs). */
export function replace(node: Layout, from: number, to: number): Layout {
  if (node.kind === 'pane') {
    return node.pane === from ? leaf(to) : node;
  }
  return { ...node, a: replace(node.a, from, to), b: replace(node.b, from, to) };
}

/** Where a pane is dropped on another one. */
export type DropZone = 'left' | 'right' | 'top' | 'bottom' | 'center';

/** Picks the zone from the pointer position within the target (0–1). */
export function dropZone(x: number, y: number): DropZone {
  const edge = 0.25;
  const distances: [DropZone, number][] = [
    ['left', x],
    ['right', 1 - x],
    ['top', y],
    ['bottom', 1 - y],
  ];
  const [zone, d] = distances.reduce((best, cur) => (cur[1] < best[1] ? cur : best));
  return d < edge ? zone : 'center';
}

/** Removes a pane; its sibling takes the parent's place. Null if it was the last. */
export function remove(node: Layout, target: number): Layout | null {
  if (node.kind === 'pane') {
    return node.pane === target ? null : node;
  }
  const a = remove(node.a, target);
  const b = remove(node.b, target);
  if (!a) {
    return b;
  }
  if (!b) {
    return a;
  }
  return { ...node, a, b };
}

export function setRatio(node: Layout, path: string, ratio: number): Layout {
  if (node.kind === 'pane') {
    return node;
  }
  if (path === '') {
    return { ...node, ratio: Math.min(MAX_RATIO, Math.max(MIN_RATIO, ratio)) };
  }
  const [step, rest] = [path[0], path.slice(1)];
  return step === 'a' ? { ...node, a: setRatio(node.a, rest, ratio) } : { ...node, b: setRatio(node.b, rest, ratio) };
}

/** Pane rectangles and divider positions, in percent. */
export function arrange(node: Layout): { panes: Map<number, Rect>; dividers: Divider[] } {
  const panes = new Map<number, Rect>();
  const dividers: Divider[] = [];
  const walk = (n: Layout, r: Rect, path: string) => {
    if (n.kind === 'pane') {
      panes.set(n.pane, r);
      return;
    }
    if (n.dir === 'row') {
      const w = r.w * n.ratio;
      walk(n.a, { ...r, w }, path + 'a');
      walk(n.b, { ...r, x: r.x + w, w: r.w - w }, path + 'b');
      dividers.push({ path, dir: n.dir, area: r, rect: { x: r.x + w, y: r.y, w: 0, h: r.h } });
    } else {
      const h = r.h * n.ratio;
      walk(n.a, { ...r, h }, path + 'a');
      walk(n.b, { ...r, y: r.y + h, h: r.h - h }, path + 'b');
      dividers.push({ path, dir: n.dir, area: r, rect: { x: r.x, y: r.y + h, w: r.w, h: 0 } });
    }
  };
  walk(node, { x: 0, y: 0, w: 100, h: 100 }, '');
  return { panes, dividers };
}

/** The pane next to `from` in a direction, by rectangle centers. */
export function neighbour(node: Layout, from: number, dir: 'left' | 'right' | 'up' | 'down'): number | null {
  const { panes } = arrange(node);
  const src = panes.get(from);
  if (!src) {
    return null;
  }
  const cx = src.x + src.w / 2;
  const cy = src.y + src.h / 2;
  let best: number | null = null;
  let bestDist = Infinity;
  for (const [pane, r] of panes) {
    if (pane === from) {
      continue;
    }
    const x = r.x + r.w / 2;
    const y = r.y + r.h / 2;
    const ok =
      (dir === 'left' && r.x + r.w <= src.x + 0.01) ||
      (dir === 'right' && r.x >= src.x + src.w - 0.01) ||
      (dir === 'up' && r.y + r.h <= src.y + 0.01) ||
      (dir === 'down' && r.y >= src.y + src.h - 0.01);
    const dist = Math.hypot(x - cx, y - cy);
    if (ok && dist < bestDist) {
      best = pane;
      bestDist = dist;
    }
  }
  return best;
}
