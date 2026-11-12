import { Component, inject, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { DomSanitizer } from '@angular/platform-browser';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';

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
  private readonly transloco = inject(TranslocoService);
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

  protected readonly reading = signal(false);
  protected readonly fileError = signal<string | null>(null);

  /** A Word, PDF, Markdown or HTML file: its content becomes the procedure, to check before saving. */
  protected async onFile(ev: Event): Promise<void> {
    const input = ev.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = '';
    if (!file) {
      return;
    }
    this.reading.set(true);
    this.fileError.set(null);
    try {
      const { importDocument } = await import('./document-import');
      const doc = await importDocument(file);
      this.markdown = doc.markdown;
      this.title ||= /^#{1,6} (.+)$/m.exec(doc.markdown)?.[1] ?? doc.title;
    } catch (err) {
      this.fileError.set(this.transloco.translate('procedures.web.fileFailed', { name: file.name, detail: err instanceof Error ? err.message : String(err) }));
    } finally {
      this.reading.set(false);
    }
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
