import type { disk } from '../../wailsjs/go/models'
import { formatBytes } from './format'

type T = (key: string, params?: Record<string, string | number>) => string

const safeBuses = ['usb', 'loop', 'virtual']

export function diskLabel(t: T, d: disk.Disk): string {
  const sys = d.isSystem ? ` [${t('disk.system')}]` : ''
  return `${d.model || d.id} — ${formatBytes(d.sizeBytes)} · ${d.bus} · ${d.id}${sys}`
}

/** Why d cannot be a source ('' if it can). */
export function sourceReason(t: T, d: disk.Disk): string {
  return d.isSystem ? t('disk.reasonSystemSource') : ''
}

/**
 * Why d cannot be written with `size` bytes ('' if it can). Mirrors the
 * backend rules (validate.Clone / validate.RestoreImage), which re-check them.
 */
export function destinationReason(t: T, d: disk.Disk, size: number, sourceId: string, devSafe: boolean): string {
  if (d.id === sourceId) return t('disk.reasonSource')
  if (d.isSystem) return t('disk.reasonSystem')
  if (d.sizeBytes < size) return t('disk.reasonTooSmall')
  if (devSafe && !safeBuses.includes(d.bus)) return t('disk.reasonDevSafe')
  return ''
}
