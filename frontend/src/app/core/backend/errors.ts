/** Values interpolated into a translated message. */
export type HashMap = Record<string, unknown>;

/** Shape of errors marshalled by core/errs on the Go side. */
export interface BackendErrorPayload {
  code: string;
  params?: HashMap;
  detail?: string;
}

export interface TranslatableError {
  key: string;
  params: HashMap;
  detail?: string;
}

/**
 * Maps a rejected binding call to a translation key under "errors.".
 * Wails puts the marshalled Go error in the `cause` field.
 */
export function toTranslatableError(err: unknown): TranslatableError {
  const cause = (err as { cause?: unknown } | null)?.cause;
  if (isPayload(cause)) {
    return { key: `errors.${cause.code}`, params: cause.params ?? {}, detail: cause.detail };
  }
  return { key: 'errors.internal', params: {}, detail: err instanceof Error ? err.message : String(err) };
}

function isPayload(value: unknown): value is BackendErrorPayload {
  return typeof value === 'object' && value !== null && typeof (value as BackendErrorPayload).code === 'string';
}
