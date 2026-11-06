import { arrange, dropZone, grid, leaf, neighbour, panesOf, remove, replace, setRatio, split, swap } from './layout';

describe('layout', () => {
  // 1 | 2
  //   |---
  //   | 3
  const tree = split(split(leaf(1), 1, 2, 'row'), 2, 3, 'column');

  it('splits next to the target pane', () => {
    expect(panesOf(tree)).toEqual([1, 2, 3]);
    const { panes } = arrange(tree);
    expect(panes.get(1)).toEqual({ x: 0, y: 0, w: 50, h: 100 });
    expect(panes.get(2)).toEqual({ x: 50, y: 0, w: 50, h: 50 });
    expect(panes.get(3)).toEqual({ x: 50, y: 50, w: 50, h: 50 });
  });

  it('places dividers between the halves', () => {
    const { dividers } = arrange(tree);
    expect(dividers.map((d) => [d.path, d.dir, d.rect])).toEqual([
      ['b', 'column', { x: 50, y: 50, w: 50, h: 0 }],
      ['', 'row', { x: 50, y: 0, w: 0, h: 100 }],
    ]);
  });

  it('removing a pane gives its space to the sibling', () => {
    const t = remove(tree, 2)!;
    expect(panesOf(t)).toEqual([1, 3]);
    expect(arrange(t).panes.get(3)).toEqual({ x: 50, y: 0, w: 50, h: 100 });
    expect(remove(leaf(1), 1)).toBeNull();
  });

  it('clamps ratios', () => {
    const t = setRatio(tree, '', 0.99);
    expect(arrange(t).panes.get(1)?.w).toBe(90);
    const t2 = setRatio(tree, 'b', 0.25);
    expect(arrange(t2).panes.get(2)?.h).toBe(25);
  });

  it('finds neighbours by direction', () => {
    expect(neighbour(tree, 1, 'right')).toBe(2);
    expect(neighbour(tree, 3, 'left')).toBe(1);
    expect(neighbour(tree, 2, 'down')).toBe(3);
    expect(neighbour(tree, 3, 'up')).toBe(2);
    expect(neighbour(tree, 1, 'left')).toBeNull();
  });

  it('splits before the target for left and top drops', () => {
    const t = split(leaf(1), 1, 2, 'column', true);
    expect(arrange(t).panes.get(2)).toEqual({ x: 0, y: 0, w: 100, h: 50 });
    expect(arrange(t).panes.get(1)).toEqual({ x: 0, y: 50, w: 100, h: 50 });
  });

  it('swaps and replaces panes', () => {
    expect(panesOf(swap(tree, 1, 3))).toEqual([3, 2, 1]);
    expect(panesOf(replace(tree, 2, 9))).toEqual([1, 9, 3]);
  });

  it('maps pointer positions to drop zones', () => {
    expect(dropZone(0.1, 0.5)).toBe('left');
    expect(dropZone(0.9, 0.5)).toBe('right');
    expect(dropZone(0.5, 0.05)).toBe('top');
    expect(dropZone(0.5, 0.95)).toBe('bottom');
    expect(dropZone(0.5, 0.5)).toBe('center');
  });

  it('arranges panes in a grid', () => {
    const rects = arrange(grid([1, 2, 3, 4])).panes;
    expect(rects.get(1)).toEqual({ x: 0, y: 0, w: 50, h: 50 });
    expect(rects.get(4)).toEqual({ x: 50, y: 50, w: 50, h: 50 });
    expect(panesOf(grid([1, 2, 3]))).toEqual([1, 2, 3]);
    expect(arrange(grid([7])).panes.get(7)).toEqual({ x: 0, y: 0, w: 100, h: 100 });
  });
});
