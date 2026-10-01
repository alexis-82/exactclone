<script lang="ts">
  import { t, lang, errorParts } from '../i18n/i18n'
  import { formatBytes, formatDuration } from './format'
  import type { JobState } from './job'

  let { job, oncancel, onclose }: { job: JobState; oncancel: () => void; onclose: () => void } = $props()

  const err = $derived(job.status === 'failed' ? errorParts($lang, job.error) : null)
  const verified = $derived(job.status === 'done' && job.kind === 'clone' && job.phases.includes('verify'))
</script>

<div class="backdrop">
  <div class="dialog" role="dialog" aria-modal="true" aria-live="polite">
    {#if job.status === 'running'}
      <h2>{$t(`progress.${job.kind}`)}</h2>
      {#if job.snap && job.snap.total === 0}
        <p class="phase">{$t(`progress.phase.${job.phase}`)}…</p>
        <progress></progress>
      {:else if job.snap}
        <p class="phase">{$t(`progress.phase.${job.phase}`)} — {job.snap.percent.toFixed(1)}%</p>
        <progress max="100" value={job.snap.percent}></progress>
        <p>{$t('progress.bytes', { done: formatBytes(job.snap.done), total: formatBytes(job.snap.total) })}</p>
        <p>
          {$t('progress.speed', { speed: formatBytes(job.snap.bytesPerSec) })} ·
          {$t('progress.eta', { eta: formatDuration(job.snap.etaSeconds) })}
        </p>
      {:else}
        <progress></progress>
      {/if}
      <div class="actions">
        <button class="danger-btn" disabled={job.canceling} onclick={oncancel}>
          {job.canceling ? $t('progress.canceling') : $t('progress.cancel')}
        </button>
      </div>
    {:else}
      {#if job.status === 'done'}
        <h2 class="ok">{$t('result.done')}</h2>
        {#if verified}<p class="ok">{$t('result.verifyOk')}</p>{/if}
      {:else if job.status === 'canceled'}
        <h2 class="warn">{$t('result.canceled')}</h2>
      {:else if err}
        <h2 class="danger">{$t('result.failed')}</h2>
        <p>{err.message}</p>
        {#if err.detail}<p class="detail">{err.detail}</p>{/if}
      {/if}
      {#each job.notices as notice}
        <p class="notice">{$t(`notices.${notice}`)}</p>
      {/each}
      {#if job.warnings.length > 0}
        <details>
          <summary>{$t('result.warnings', { count: job.warnings.length })}</summary>
          <ul class="warnings">
            {#each job.warnings as w}
              <li><code>{w.path}</code>: {w.reason}</li>
            {/each}
          </ul>
        </details>
      {/if}
      <div class="actions">
        <button class="primary" onclick={onclose}>{$t('result.close')}</button>
      </div>
    {/if}
  </div>
</div>

<style>
  progress {
    width: 100%;
    height: 18px;
  }
  .phase {
    font-weight: 600;
  }
  .detail {
    font-family: monospace;
    font-size: 12px;
    color: var(--muted);
    word-break: break-all;
  }
  .notice {
    background: var(--notice-bg);
    border-left: 4px solid var(--warn);
    padding: 8px 12px;
  }
  .warnings {
    max-height: 200px;
    overflow: auto;
    font-size: 12px;
  }
</style>
