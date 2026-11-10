import { TestBed } from '@angular/core/testing';
import { signal } from '@angular/core';

import { ContextMenuService } from '../shared/context-menu.service';
import { TerminalPrefs } from '../terminal/terminal-prefs';
import { SessionPrefs } from './session-prefs';
import { SessionsStore } from './sessions.store';
import { Workspace } from './workspace';

/** A pointer event with the fields the drag reads (jsdom has no PointerEvent). */
function pointer(type: string, x: number, y: number, target: EventTarget = document): Event {
  const ev = new MouseEvent(type, { bubbles: true, cancelable: true, clientX: x, clientY: y, button: 0 });
  Object.defineProperty(ev, 'pointerId', { value: 1 });
  Object.defineProperty(ev, 'target', { value: target });
  return ev;
}

describe('Workspace pane dragging', () => {
  let moves: unknown[][];
  let active: ReturnType<typeof signal<number>>;
  let ws: any;
  let header: HTMLElement;
  let paneB: HTMLElement;
  let tab2: HTMLElement;
  let bar: HTMLElement;
  let under: HTMLElement | null;

  beforeEach(() => {
    vi.useFakeTimers();
    moves = [];
    active = signal(1);
    const store = {
      tabs: () => [],
      panes: () => [],
      activeKey: active,
      activate: (k: number) => active.set(k),
      pane: (k: number) => ({ key: k, title: k === 1 ? 'web-1' : 'db-1' }),
      movePane: (...a: unknown[]) => moves.push(['pane', ...a]),
      movePaneToTab: (...a: unknown[]) => moves.push(['tab', ...a]),
      movePaneToNewTab: (...a: unknown[]) => moves.push(['new', ...a]),
    };
    TestBed.configureTestingModule({
      providers: [
        { provide: SessionsStore, useValue: store },
        { provide: SessionPrefs, useValue: {} },
        { provide: TerminalPrefs, useValue: {} },
        { provide: ContextMenuService, useValue: {} },
      ],
    });
    ws = TestBed.runInInjectionContext(() => new Workspace());

    document.body.innerHTML = `
      <div class="tabs"><div data-tab-key="1"></div><div data-tab-key="2" id="tab2"></div></div>
      <div data-pane-key="1"><header class="pane-header"><span id="title">web-1</span><button id="btn">x</button></header></div>
      <div data-pane-key="2" id="paneB"></div>`;
    header = document.getElementById('title')!;
    paneB = document.getElementById('paneB')!;
    tab2 = document.getElementById('tab2')!;
    bar = document.querySelector('.tabs')!;
    paneB.getBoundingClientRect = () => ({ left: 0, top: 0, width: 100, height: 100, right: 100, bottom: 100, x: 0, y: 0, toJSON: () => '' });
    under = null;
    document.elementFromPoint = () => under;
  });

  afterEach(() => {
    ws.cancelDrag();
    vi.useRealTimers();
  });

  const start = (target: HTMLElement = header) => ws.onPanePointerDown(pointer('pointerdown', 0, 0, target), 1);

  it('drops a pane on the side of another pane', () => {
    start();
    under = paneB;
    document.dispatchEvent(pointer('pointermove', 95, 50));
    expect(ws.dropHint()).toEqual({ target: 2, zone: 'right' });
    expect(ws.dragGhost().title).toBe('web-1');
    expect(document.body.classList.contains('pane-dragging')).toBe(true);
    document.dispatchEvent(pointer('pointerup', 95, 50));
    expect(moves).toEqual([['pane', 1, 2, 'right']]);
    expect(document.body.classList.contains('pane-dragging')).toBe(false);
  });

  it('a click without moving is not a drag, and buttons in the header do not start one', () => {
    start();
    under = paneB;
    document.dispatchEvent(pointer('pointermove', 2, 2));
    document.dispatchEvent(pointer('pointerup', 2, 2));
    expect(moves).toEqual([]);
    start(document.getElementById('btn')!);
    document.dispatchEvent(pointer('pointermove', 95, 50));
    document.dispatchEvent(pointer('pointerup', 95, 50));
    expect(moves).toEqual([]);
  });

  it('opens a tab hovered for a moment, then drops in it', () => {
    start();
    under = tab2;
    document.dispatchEvent(pointer('pointermove', 40, 5));
    expect(ws.dropTab()).toBe(2);
    expect(active()).toBe(1);
    vi.advanceTimersByTime(700);
    expect(active()).toBe(2);
    // Now over a pane of that tab: dropped at a precise place.
    under = paneB;
    document.dispatchEvent(pointer('pointermove', 50, 5));
    expect(ws.dropTab()).toBeNull();
    document.dispatchEvent(pointer('pointerup', 50, 5));
    expect(moves).toEqual([['pane', 1, 2, 'top']]);
  });

  it('drops on a tab or on the empty bar, and Escape cancels', () => {
    start();
    under = tab2;
    document.dispatchEvent(pointer('pointermove', 40, 5));
    document.dispatchEvent(pointer('pointerup', 40, 5));
    start();
    under = bar;
    document.dispatchEvent(pointer('pointermove', 300, 5));
    document.dispatchEvent(pointer('pointerup', 300, 5));
    expect(moves).toEqual([['tab', 1, 2], ['new', 1]]);
    start();
    under = paneB;
    document.dispatchEvent(pointer('pointermove', 95, 50));
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    document.dispatchEvent(pointer('pointerup', 95, 50));
    expect(moves).toHaveLength(2);
  });
});
