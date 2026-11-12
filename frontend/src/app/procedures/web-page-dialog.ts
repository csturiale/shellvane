import { Component, inject, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { DomSanitizer } from '@angular/platform-browser';
import { TranslocoDirective } from '@jsverse/transloco';

import { writeClipboard } from '../core/clipboard';
import { Modal } from '../shared/modal';
import { bookmarklet } from './bookmarklet';
import { htmlToMarkdown } from './html-to-markdown';

/**
 * Procedures from web pages (Confluence, a wiki behind single sign-on):
 * the "Send to Shellvane" bookmarklet, or a page pasted here.
 */
@Component({
  selector: 'app-procedure-web-dialog',
  imports: [FormsModule, TranslocoDirective, Modal],
  templateUrl: './web-page-dialog.html',
  styleUrls: ['../shared/form.scss', './web-page-dialog.scss'],
})
export class WebPageDialog {
  readonly saved = output<{ title: string; markdown: string }>();
  readonly closed = output<void>();

  private readonly sanitizer = inject(DomSanitizer);
  private readonly code = bookmarklet();
  /** The bookmarklet as a link to drag to the bookmarks bar. */
  protected readonly link = this.sanitizer.bypassSecurityTrustUrl(this.code);
  protected readonly copied = signal(false);
  protected title = '';
  protected markdown = '';

  protected async copyCode(): Promise<void> {
    await writeClipboard(this.code);
    this.copied.set(true);
  }

  /** A pasted page keeps its formatting: its HTML becomes Markdown. */
  protected onPaste(ev: ClipboardEvent): void {
    const html = ev.clipboardData?.getData('text/html') ?? '';
    if (!html) {
      return; // plain text: pasted as it is
    }
    ev.preventDefault();
    const doc = new DOMParser().parseFromString(html, 'text/html');
    this.markdown = htmlToMarkdown(doc.body);
    if (!this.title) {
      this.title = /^#{1,6} (.+)$/m.exec(this.markdown)?.[1] ?? doc.title ?? '';
    }
  }

  protected save(): void {
    if (this.markdown.trim()) {
      this.saved.emit({ title: this.title.trim(), markdown: this.markdown });
    }
  }
}
