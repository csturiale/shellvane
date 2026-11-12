import { Component, computed, inject } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective } from '@jsverse/transloco';
import { LucideTrash2 } from '@lucide/angular';

import { PersonalValues } from '../core/personal-values';
import { HelpLink } from '../guide/help-link';

/** Settings › Personal values: the values used everywhere, by name. */
@Component({
  selector: 'app-personal-values-settings',
  imports: [FormsModule, TranslocoDirective, LucideTrash2, HelpLink],
  templateUrl: './personal-values-settings.html',
  styleUrl: './personal-values-settings.scss',
})
export class PersonalValuesSettings {
  protected readonly personal = inject(PersonalValues);
  protected readonly entries = computed(() =>
    Object.entries(this.personal.global())
      .map(([name, value]) => ({ name, value }))
      .sort((a, b) => a.name.localeCompare(b.name)),
  );
  protected newName = '';
  protected newValue = '';

  protected change(name: string, ev: Event): void {
    this.personal.setGlobal(name, (ev.target as HTMLInputElement).value);
  }

  protected add(): void {
    const name = this.newName.trim();
    if (!/^[A-Za-z_][\w-]*$/.test(name)) {
      return;
    }
    this.personal.setGlobal(name, this.newValue);
    this.newName = '';
    this.newValue = '';
  }
}
