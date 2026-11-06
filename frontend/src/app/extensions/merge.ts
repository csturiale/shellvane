import { Translation } from '@jsverse/transloco';

/** Deep-merges b over a (objects only; other values from b win). */
export function mergeTranslations(a: Translation, b: Translation): Translation {
  const out: Translation = { ...a };
  for (const [k, v] of Object.entries(b)) {
    const prev = out[k];
    out[k] = isObject(prev) && isObject(v) ? mergeTranslations(prev, v) : v;
  }
  return out;
}

function isObject(v: unknown): v is Translation {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}
