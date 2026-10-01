// Verifies that it.json and en.json have the same keys and that every
// translation key used in the sources exists.
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { join } from 'node:path'

const root = new URL('../src/', import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, '$1')
const it = JSON.parse(readFileSync(join(root, 'i18n/it.json'), 'utf8'))
const en = JSON.parse(readFileSync(join(root, 'i18n/en.json'), 'utf8'))
const problems = []

for (const k of Object.keys(it)) if (!(k in en)) problems.push(`missing in en.json: ${k}`)
for (const k of Object.keys(en)) if (!(k in it)) problems.push(`missing in it.json: ${k}`)

function files(dir) {
  return readdirSync(dir).flatMap((f) => {
    const p = join(dir, f)
    return statSync(p).isDirectory() ? files(p) : /\.(svelte|ts)$/.test(f) ? [p] : []
  })
}

// Keys built at runtime: their prefix must exist in the dictionary.
const dynamicPrefixes = ['progress.', 'progress.phase.', 'notices.', 'errors.']
for (const f of files(root).filter((f) => !f.endsWith('i18n.ts'))) {
  const src = readFileSync(f, 'utf8')
  for (const m of src.matchAll(/\$t\(\s*'([^']+)'/g)) {
    if (!(m[1] in it)) problems.push(`${f}: unknown key ${m[1]}`)
  }
  for (const m of src.matchAll(/\$t\(\s*`([^`$]+)\$\{/g)) {
    if (!dynamicPrefixes.includes(m[1])) problems.push(`${f}: unexpected dynamic key prefix ${m[1]}`)
  }
}

// Codes emitted by the backend must be translated.
const goRoot = join(root, '../../')
const goFiles = ['app.go', 'jobs.go', 'config.go', 'internal/validate/validate.go']
for (const f of goFiles) {
  const src = readFileSync(join(goRoot, f), 'utf8')
  for (const m of src.matchAll(/coded\("([a-z0-9_]+)"/g)) {
    if (!(`errors.${m[1]}` in it)) problems.push(`${f}: untranslated error code ${m[1]}`)
  }
  for (const m of src.matchAll(/Code\w+\s*=\s*"([a-z0-9_]+)"/g)) {
    if (!(`errors.${m[1]}` in it)) problems.push(`${f}: untranslated error code ${m[1]}`)
  }
  for (const m of src.matchAll(/Notice\w+\s*=\s*"([a-z0-9_]+)"/g)) {
    if (!(`notices.${m[1]}` in it)) problems.push(`${f}: untranslated notice ${m[1]}`)
  }
}
for (const phase of ['copy', 'verify', 'scan', 'archive', 'restore']) {
  if (!(`progress.phase.${phase}` in it)) problems.push(`missing progress.phase.${phase}`)
}
for (const kind of ['clone', 'archive', 'restore']) {
  if (!(`progress.${kind}` in it)) problems.push(`missing progress.${kind}`)
}

if (problems.length) {
  console.error(problems.join('\n'))
  process.exit(1)
}
console.log(`i18n OK: ${Object.keys(it).length} keys in it/en`)
