import { Type } from '@angular/core';
import { Translation } from '@jsverse/transloco';

import { MenuItem } from '../shared/context-menu.service';

/** What a terminal pane tells extensions when its context menu opens. */
export interface TerminalMenuContext {
  /** The selected text (empty when nothing is selected). */
  selection: string;
  connected: boolean;
}

/** Adds entries to a terminal pane's context menu. */
export interface TerminalMenuContributor {
  items(ctx: TerminalMenuContext): MenuItem[];
}

/** Objects that can be shared with others (host groups apart). */
export type ShareKind = 'environment' | 'snippet' | 'workspace' | 'rule';

/** A command the palette offers; label is a translation key. */
export interface PaletteCommand {
  label: string;
  detail: string;
  run: () => void;
}

/** A login a host can take from somewhere other than the vault. */
export interface CredentialSource {
  ref: string;
  label: string;
  group: string;
}

/**
 * Sharing objects with other people (for example a team). The public code
 * asks it, and shows its components where objects are listed and edited.
 */
export interface SharingProvider {
  /** Once the vault is open (not in detached windows). */
  start(): void;
  /** A shared object whose commands changed and wait for approval. */
  needsReview(kind: ShareKind | 'group', localId: string): boolean;
  /** Entries of a host group's context menu; run shows the errors of work where the menu is. */
  groupMenu(group: { id: string; name: string }, run: (work: () => Promise<unknown>) => void): (MenuItem | null)[];
  /** Commands for the palette. */
  paletteCommands(): PaletteCommand[];
  /** Logins hosts can use. */
  credentialSources(): Promise<CredentialSource[]>;
}

/**
 * Components of a sharing extension, rendered with NgComponentOutlet:
 * - button: inputs kind, localId, name, onFailed(message);
 * - note: inputs kind (or 'group'), localId;
 * - review: inputs kind (or 'group'), localId, onClosed(), onApproved();
 * - groupBadge: input groupId.
 */
export interface SharingExtension {
  provider: Type<SharingProvider>;
  button: Type<unknown>;
  note: Type<unknown>;
  review: Type<unknown>;
  groupBadge: Type<unknown>;
}

/** A full-page view with its own entry in the activity bar. */
export interface ExtensionView {
  id: string;
  /** Translation key of its name. */
  label: string;
  /** A component that draws the icon (22 px). */
  icon: Type<unknown>;
  /** Loads the view's component (a lazy chunk, so the app starts fast). */
  load: () => Promise<Type<unknown>>;
}

/**
 * A feature distributed apart from the community edition. It plugs into the
 * app only through these points; the community edition has none.
 */
export interface AppExtension {
  name: string;
  /** Sections shown in Settings, after the built-in ones (loaded when Settings opens). */
  settings?: (() => Promise<Type<unknown>>)[];
  /** Components rendered once at the root of the app (dialogs, overlays). */
  root?: Type<unknown>[];
  /** An injectable that adds entries to a terminal pane's context menu. */
  terminalMenu?: Type<TerminalMenuContributor>;
  /** Guide topics (public/guide/<lang>/<id>.md), after the built-in ones. */
  guideTopics?: string[];
  /** Sharing objects with others. At most one extension provides it. */
  sharing?: SharingExtension;
  /** Full-page views, after the built-in sections. */
  views?: ExtensionView[];
  /** Translations merged over the app's own for a language. */
  translations?: (lang: string) => Promise<Translation>;
}
