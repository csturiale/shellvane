import { Component, input, output } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';
import {
  LucideBookOpen,
  LucideCable,
  LucideFolderSync,
  LucideKeyRound,
  LucideLayers,
  LucidePanelLeftClose,
  LucidePanelLeftOpen,
  LucidePuzzle,
  LucideServer,
  LucideSettings,
  LucideSquareCode,
  LucideStore,
  LucideZap,
} from '@lucide/angular';
import { combo } from '../sessions/shortcuts';

import { EXTENSION_VIEWS, SECTIONS, View } from './views';
import { NgComponentOutlet } from '@angular/common';

@Component({
  selector: 'app-activity-bar',
  imports: [
    NgComponentOutlet,
    TranslocoDirective,
    LucideSettings,
    LucideBookOpen,
    LucidePanelLeftClose,
    LucidePanelLeftOpen,
    LucideServer,
    LucideLayers,
    LucideSquareCode,
    LucideCable,
    LucideFolderSync,
    LucideKeyRound,
    LucideZap,
    LucidePuzzle,
    LucideStore,
  ],
  templateUrl: './activity-bar.html',
  styleUrl: './activity-bar.scss',
})
export class ActivityBar {
  protected readonly panelKeys = combo('L');
  readonly active = input.required<View>();
  readonly select = output<View>();
  /** A new version has news the user has not opened yet. */
  readonly news = input(false);
  /** The side panel is hidden; the toggle shows it again. */
  readonly collapsed = input(false);
  /** False on full-page views, which have no side panel. */
  readonly hasPanel = input(true);
  readonly togglePanel = output<void>();
  /** The community gallery dialog is open. */
  readonly galleryOpen = input(false);
  readonly gallery = output<void>();

  protected readonly sections = SECTIONS;
  protected readonly extensionViews = EXTENSION_VIEWS;
}
