const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']

export function formatBytes(n: number): string {
  let i = 0
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024
    i++
  }
  return `${n.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

export function formatDuration(seconds: number): string {
  if (seconds < 0 || !isFinite(seconds)) return '—'
  const s = Math.round(seconds)
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  const pad = (x: number) => String(x).padStart(2, '0')
  return h > 0 ? `${h}:${pad(m)}:${pad(sec)}` : `${m}:${pad(sec)}`
}

/** Folder containing a file path (Windows or Unix separators). */
export function dirname(path: string): string {
  const i = Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\'))
  if (i <= 0) return path
  if (i === 2 && path[1] === ':') return path.slice(0, 3) // C:\
  return path.slice(0, i)
}
