import { EXTENSIONS } from '../extensions/extensions';

/** Sections reachable from the activity bar; settings is pinned at the bottom. */
export const SECTIONS = ['hosts', 'environments', 'snippets', 'tunnels', 'files', 'keys', 'automation', 'plugins'] as const;
export type Section = (typeof SECTIONS)[number];
/** A built-in view, or the id of an extension's view. */
export type View = Section | 'settings' | 'guide' | 'recordings' | (string & {});

/** Views of extensions, after the built-in sections. */
export const EXTENSION_VIEWS = EXTENSIONS.flatMap((e) => e.views ?? []);

/** Views that use the whole work area, without a side panel. */
export const FULL_VIEWS: readonly View[] = ['settings', 'files', 'automation', 'guide', 'recordings', 'plugins', ...EXTENSION_VIEWS.map((v) => v.id)];
