import { writable } from 'svelte/store'

export type JobKind = 'clone' | 'archive' | 'restore'
export type JobStatus = 'idle' | 'running' | 'done' | 'failed' | 'canceled'

export interface Snapshot {
  done: number
  total: number
  percent: number
  bytesPerSec: number
  etaSeconds: number
}

export interface Warning {
  path: string
  reason: string
}

export interface JobState {
  status: JobStatus
  kind: JobKind | null
  phase: string
  snap: Snapshot | null
  phases: string[]
  error: string
  notices: string[]
  warnings: Warning[]
  canceling: boolean
}

export const idleJob: JobState = {
  status: 'idle', kind: null, phase: '', snap: null, phases: [], error: '', notices: [], warnings: [], canceling: false,
}

export const job = writable<JobState>(idleJob)

/** Called right after a Start* binding succeeded, before the first event. */
export function jobStarted(kind: JobKind) {
  job.set({ ...idleJob, status: 'running', kind })
}
