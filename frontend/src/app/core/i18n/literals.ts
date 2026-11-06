/**
 * Inserts literal text into a translation. Transloco interpolates `{{...}}`
 * in parameter values too, so texts such as `{{name}}` are passed as
 * markers and put back after translating.
 */
export class Literals {
  readonly params: Record<string, string>;

  constructor(private readonly values: Record<string, string>) {
    this.params = Object.fromEntries(Object.keys(values).map((k) => [k, `⁠${k}⁠`]));
  }

  fill(text: string): string {
    let out = text;
    for (const [k, v] of Object.entries(this.values)) {
      out = out.split(`⁠${k}⁠`).join(v);
    }
    return out;
  }
}
