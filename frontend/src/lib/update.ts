import { writable } from 'svelte/store'
import { CheckUpdate } from '../../wailsjs/go/main/App'
import type { main } from '../../wailsjs/go/models'

/** Update offered to the user (shown by UpdateDialog). */
export const updateOffer = writable<main.UpdateInfo | null>(null)

/** Download progress of the update being installed. */
export const updateProgress = writable<{ done: number; total: number } | null>(null)

/** Startup check: silent when offline, when there is no release or no update. */
export async function checkUpdatesInBackground() {
  try {
    const info = await CheckUpdate()
    if (info.available) updateOffer.set(info)
  } catch {
    // offline or GitHub unavailable: try again at the next start
  }
}
