import { derived, writable } from 'svelte/store'
import it from './it.json'
import en from './en.json'

export type Lang = 'it' | 'en'

const dictionaries: Record<Lang, Record<string, string>> = { it, en }

export const lang = writable<Lang>('it')

export type Params = Record<string, string | number>

function translate(l: Lang, key: string, params?: Params): string {
  let text = dictionaries[l][key] ?? dictionaries.it[key] ?? key
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      text = text.replaceAll(`{${k}}`, String(v))
    }
  }
  return text
}

/** $t('key', {param}) translates in the current language. */
export const t = derived(lang, (l) => (key: string, params?: Params) => translate(l, key, params))

/**
 * Backend errors have the form "code" or "code|detail".
 * Returns the translated message and the technical detail.
 */
export function errorParts(l: Lang, err: unknown): { message: string; detail: string } {
  const raw = typeof err === 'string' ? err : err instanceof Error ? err.message : String(err)
  const sep = raw.indexOf('|')
  const code = sep >= 0 ? raw.slice(0, sep) : raw
  const detail = sep >= 0 ? raw.slice(sep + 1) : ''
  const key = `errors.${code}`
  if (dictionaries[l][key]) {
    return { message: translate(l, key), detail }
  }
  return { message: translate(l, 'errors.unknown'), detail: raw }
}
