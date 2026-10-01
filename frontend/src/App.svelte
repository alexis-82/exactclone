<script lang="ts">
  import { onMount } from 'svelte'
  import { t, lang, type Lang } from './i18n/i18n'
  import { Cancel, GetConfig, IsDevSafe, IsElevated, SetLanguage } from '../wailsjs/go/main/App'
  import { EventsOn } from '../wailsjs/runtime/runtime'
  import BackupTab from './lib/BackupTab.svelte'
  import RestoreTab from './lib/RestoreTab.svelte'
  import ProgressPanel from './lib/ProgressPanel.svelte'
  import { idleJob, job, type JobState, type Snapshot, type Warning } from './lib/job'

  let tab = $state<'backup' | 'restore'>('backup')
  let elevated = $state(true)
  let devSafe = $state(false)
  let ready = $state(false)

  async function changeLanguage(l: Lang) {
    lang.set(l)
    try {
      await SetLanguage(l)
    } catch {
      // the language still applies to this session
    }
  }

  function cancelJob() {
    job.update((j) => ({ ...j, canceling: true }))
    Cancel()
  }

  onMount(() => {
    const offProgress = EventsOn('job:progress', (e: { kind: JobState['kind']; phase: string } & Snapshot) => {
      job.update((j) => ({
        ...j,
        status: 'running',
        kind: e.kind,
        phase: e.phase,
        snap: e,
        phases: j.phases.includes(e.phase) ? j.phases : [...j.phases, e.phase],
      }))
    })
    const offDone = EventsOn(
      'job:done',
      (e: { kind: JobState['kind']; status: JobState['status']; error?: string; notices?: string[]; warnings?: Warning[] }) => {
        job.update((j) => ({
          ...j,
          status: e.status,
          kind: e.kind,
          error: e.error ?? '',
          notices: e.notices ?? [],
          warnings: e.warnings ?? [],
          canceling: false,
        }))
      },
    )
    Promise.all([GetConfig(), IsElevated(), IsDevSafe()]).then(([cfg, el, ds]) => {
      lang.set(cfg.language === 'en' ? 'en' : 'it')
      elevated = el
      devSafe = ds
      ready = true
    })
    return () => {
      offProgress()
      offDone()
    }
  })
</script>

<header>
  <h1>{$t('app.title')}</h1>
  <label class="lang">
    {$t('app.language')}
    <select value={$lang} onchange={(e) => changeLanguage(e.currentTarget.value as Lang)}>
      <option value="it">Italiano</option>
      <option value="en">English</option>
    </select>
  </label>
</header>

{#if ready && !elevated}
  <div class="banner danger" role="alert">{$t('banner.notElevated')}</div>
{/if}
{#if devSafe}
  <div class="banner warn">{$t('banner.devSafe')}</div>
{/if}

<div class="tabs" role="tablist">
  <button role="tab" aria-selected={tab === 'backup'} class:active={tab === 'backup'} onclick={() => (tab = 'backup')}>
    {$t('tabs.backup')}
  </button>
  <button role="tab" aria-selected={tab === 'restore'} class:active={tab === 'restore'} onclick={() => (tab = 'restore')}>
    {$t('tabs.restore')}
  </button>
</div>

<main>
  {#if ready}
    <div hidden={tab !== 'backup'}><BackupTab {elevated} {devSafe} /></div>
    <div hidden={tab !== 'restore'}><RestoreTab /></div>
  {/if}
</main>

{#if $job.status !== 'idle'}
  <ProgressPanel job={$job} oncancel={cancelJob} onclose={() => job.set(idleJob)} />
{/if}
