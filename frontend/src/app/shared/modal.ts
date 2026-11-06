import { Component, ElementRef, afterNextRender, input, output, viewChild } from '@angular/core';

/**
 * Modal built on the native <dialog> element: focus trapping, Escape to close
 * and the backdrop come from the platform. Render it with @if to open it.
 */
@Component({
  selector: 'app-modal',
  templateUrl: './modal.html',
  styleUrl: './modal.scss',
})
export class Modal {
  readonly heading = input.required<string>();
  /** Room for tables (up to 1000px instead of 480px). */
  readonly wide = input(false);
  readonly closed = output<void>();

  private static nextId = 0;
  protected readonly titleId = `modal-title-${Modal.nextId++}`;
  private readonly dialog = viewChild.required<ElementRef<HTMLDialogElement>>('dialog');

  constructor() {
    afterNextRender(() => this.dialog().nativeElement.showModal());
  }

  protected onCancel(event: Event): void {
    // Escape: let the owner remove the modal instead of closing it natively.
    event.preventDefault();
    this.closed.emit();
  }
}
