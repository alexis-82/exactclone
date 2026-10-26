import DOMPurify from 'dompurify'
import { marked } from 'marked'

/** Renders Markdown to sanitized HTML. Always sanitize: injected scripts would reach the Go bindings. */
export function renderMarkdown(src: string): string {
  const html = marked.parse(src, { async: false, gfm: true, breaks: true }) as string
  return DOMPurify.sanitize(html)
}

/** Only http(s) links with a host and mailto links may be opened (opened with BrowserOpenURL). */
export function isSafeUrl(u: string): boolean {
  try {
    const url = new URL(u.trim())
    if (url.protocol === 'http:' || url.protocol === 'https:') return url.host !== ''
    return url.protocol === 'mailto:'
  } catch {
    return false
  }
}
