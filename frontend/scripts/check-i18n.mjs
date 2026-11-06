// Fails when a translation file does not define exactly the keys of en.json.
// Usage: node scripts/check-i18n.mjs [dir]
import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';

const dir = process.argv[2] ?? 'src/i18n';
const source = 'en.json';

function flatten(obj, prefix = '') {
  return Object.entries(obj).flatMap(([k, v]) =>
    v !== null && typeof v === 'object' ? flatten(v, `${prefix}${k}.`) : [`${prefix}${k}`],
  );
}

let failed = false;
const sourceKeys = checkDir(dir);
// Strings of features outside the community edition live in pro/ and its
// subfolders (one per feature), each with the same languages.
const proDirs = (d) => (existsSync(d) ? [d, ...readdirSync(d, { withFileTypes: true }).filter((e) => e.isDirectory()).flatMap((e) => proDirs(join(d, e.name)))] : []);
for (const d of proDirs(join(dir, 'pro'))) {
  if (readdirSync(d).includes(source)) checkDir(d);
}

function checkDir(dir) {
  const load = (file) => new Set(flatten(JSON.parse(readFileSync(join(dir, file), 'utf8'))));
  const sourceKeys = load(source);
  for (const file of readdirSync(dir).filter((f) => f.endsWith('.json') && f !== source)) {
    const keys = load(file);
    const missing = [...sourceKeys].filter((k) => !keys.has(k));
    const extra = [...keys].filter((k) => !sourceKeys.has(k));
    if (missing.length || extra.length) {
      failed = true;
      console.error(`${file}:`);
      missing.forEach((k) => console.error(`  missing: ${k}`));
      extra.forEach((k) => console.error(`  extra:   ${k}`));
    }
  }
  return sourceKeys;
}

// Every guide topic must exist in every language, with the same anchors
// ({#id} on headings), so links from the app work in any language.
const guideDir = process.argv[3] ?? 'public/guide';
const anchors = (file) =>
  new Set([...readFileSync(file, 'utf8').matchAll(/\{#([\w-]+)\}/g)].map((m) => m[1]));
const topics = readdirSync(join(guideDir, 'en')).filter((f) => f.endsWith('.md'));
for (const lang of readdirSync(guideDir).filter((d) => d !== 'en')) {
  for (const topic of topics) {
    const file = join(guideDir, lang, topic);
    let found;
    try {
      found = anchors(file);
    } catch {
      failed = true;
      console.error(`guide/${lang}: missing ${topic}`);
      continue;
    }
    const missing = [...anchors(join(guideDir, 'en', topic))].filter((a) => !found.has(a));
    if (missing.length) {
      failed = true;
      console.error(`guide/${lang}/${topic}: missing anchors ${missing.join(', ')}`);
    }
  }
}

if (failed) process.exit(1);
console.log(
  `i18n: all locales match ${source} (${sourceKeys.size} keys); guide: ${topics.length} topics`,
);
