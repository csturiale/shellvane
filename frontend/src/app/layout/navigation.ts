import { Injectable, signal } from '@angular/core';

import { View } from './views';

/** Which view the shell shows; shared so any panel can switch it. */
@Injectable({ providedIn: 'root' })
export class Navigation {
  readonly view = signal<View>('hosts');
  /** Host to open in the file browser next time it is shown. */
  readonly filesHost = signal<string | null>(null);
  /** Folder to show there ('' = the home folder). */
  readonly filesPath = signal('');

  /** One-shot request for a panel ("newHost", "newGroup", "import"). */
  readonly request = signal<string | null>(null);

  ask(view: View, request: string): void {
    this.view.set(view);
    this.request.set(request);
  }

  /** Returns the pending request if it matches, clearing it. */
  take(request: string): boolean {
    if (this.request() !== request) {
      return false;
    }
    this.request.set(null);
    return true;
  }

  /** The first-run tour is shown (also reopened from the guide). */
  readonly tour = signal(false);

  /** Guide topic and heading to show. */
  readonly guideTopic = signal<{ topic: string; anchor: string }>({ topic: 'getting-started', anchor: '' });

  openGuide(topic = 'getting-started', anchor = ''): void {
    this.guideTopic.set({ topic, anchor });
    this.view.set('guide');
  }

  openFiles(hostId: string, path = ''): void {
    this.filesPath.set(path);
    this.filesHost.set(hostId);
    this.view.set('files');
  }
}
