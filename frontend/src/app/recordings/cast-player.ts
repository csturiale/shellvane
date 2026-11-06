import { Component, DestroyRef, ElementRef, afterNextRender, computed, effect, inject, input, signal, untracked, viewChild } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';
import { LucidePause, LucidePlay, LucideRotateCcw } from '@lucide/angular';
import { FitAddon } from '@xterm/addon-fit';
import { Terminal } from '@xterm/xterm';

import { ThemeService } from '../core/theme/theme.service';
import { findScheme } from '../terminal/schemes';
import { TerminalPrefs } from '../terminal/terminal-prefs';
import { Cast, clock, eventsUntil, parseCast } from './cast';

export const SPEEDS = [0.5, 1, 2, 4, 8];

/** Plays an asciinema recording, or shows a log, in a read-only terminal. */
@Component({
  selector: 'app-cast-player',
  imports: [TranslocoDirective, LucidePlay, LucidePause, LucideRotateCcw],
  templateUrl: './cast-player.html',
  styleUrl: './cast-player.scss',
})
export class CastPlayer {
  /** File content and its kind. */
  readonly text = input.required<string>();
  readonly kind = input.required<'log' | 'recording'>();

  private readonly prefs = inject(TerminalPrefs);
  private readonly theme = inject(ThemeService);
  private readonly hostEl = viewChild.required<ElementRef<HTMLElement>>('screen');

  protected readonly cast = signal<Cast | null>(null);
  protected readonly error = signal(false);
  protected readonly playing = signal(false);
  protected readonly position = signal(0);
  protected readonly speed = signal(1);
  protected readonly speeds = SPEEDS;
  protected readonly clock = clock;
  protected readonly duration = computed(() => this.cast()?.duration ?? 0);

  private term?: Terminal;
  private fit?: FitAddon;
  private next = 0; // index of the next event
  private timer?: ReturnType<typeof setTimeout>;
  private startedAt = 0; // wall clock when the position was `position()`

  constructor() {
    afterNextRender(() => {
      const scheme = findScheme(this.prefs.scheme(), this.theme.isDark());
      this.term = new Terminal({
        disableStdin: true,
        convertEol: this.kind() === 'log',
        fontFamily: '"JetBrains Mono", "Cascadia Code", Menlo, Consolas, "DejaVu Sans Mono", monospace',
        fontSize: this.prefs.fontSize(),
        scrollback: 100000,
        theme: scheme.colors,
      });
      this.term.open(this.hostEl().nativeElement);
      if (this.kind() === 'log') {
        this.fit = new FitAddon();
        this.term.loadAddon(this.fit);
        this.fit.fit();
      }
      this.load();
    });
    effect(() => {
      this.text();
      untracked(() => this.term && this.load());
    });
    inject(DestroyRef).onDestroy(() => {
      clearTimeout(this.timer);
      this.term?.dispose();
    });
  }

  private load(): void {
    this.pause();
    this.term?.reset();
    this.error.set(false);
    if (this.kind() === 'log') {
      this.cast.set(null);
      this.term?.write(this.text());
      return;
    }
    try {
      const c = parseCast(this.text());
      this.cast.set(c);
      this.term?.resize(c.width, c.height);
      this.seek(0);
      this.play();
    } catch {
      this.error.set(true);
    }
  }

  protected toggle(): void {
    if (this.playing()) {
      this.pause();
    } else {
      if (this.position() >= this.duration()) {
        this.seek(0);
      }
      this.play();
    }
  }

  protected setSpeed(ev: Event): void {
    const playing = this.playing();
    this.pause();
    this.speed.set(Number((ev.target as HTMLSelectElement).value));
    if (playing) {
      this.play();
    }
  }

  protected onSeek(ev: Event): void {
    const playing = this.playing();
    this.pause();
    this.seek(Number((ev.target as HTMLInputElement).value));
    if (playing) {
      this.play();
    }
  }

  /** Redraws the screen at time t from the start. */
  protected seek(t: number): void {
    const c = this.cast();
    if (!c || !this.term) {
      return;
    }
    this.term.reset();
    this.term.resize(c.width, c.height);
    this.next = eventsUntil(c.events, t);
    for (const ev of c.events.slice(0, this.next)) {
      this.apply(ev);
    }
    this.position.set(t);
  }

  private play(): void {
    this.playing.set(true);
    this.startedAt = performance.now();
    this.schedule();
  }

  private pause(): void {
    if (this.playing()) {
      this.position.set(this.now());
    }
    this.playing.set(false);
    clearTimeout(this.timer);
  }

  /** Current position in the recording, in seconds. */
  private now(): number {
    return Math.min(this.duration(), this.position() + ((performance.now() - this.startedAt) / 1000) * this.speed());
  }

  private schedule(): void {
    const c = this.cast();
    if (!c || !this.playing()) {
      return;
    }
    const t = this.now();
    while (this.next < c.events.length && c.events[this.next].time <= t) {
      this.apply(c.events[this.next++]);
    }
    if (this.next >= c.events.length) {
      this.position.set(this.duration());
      this.playing.set(false);
      return;
    }
    const wait = ((c.events[this.next].time - t) / this.speed()) * 1000;
    this.timer = setTimeout(() => {
      this.position.set(this.now());
      this.startedAt = performance.now();
      this.schedule();
    }, Math.max(0, Math.min(wait, 250)));
  }

  private apply(ev: { kind: string; data: string }): void {
    if (ev.kind === 'o') {
      this.term?.write(ev.data);
    } else {
      const [w, h] = ev.data.split('x').map(Number);
      if (w > 0 && h > 0) {
        this.term?.resize(w, h);
      }
    }
  }
}
