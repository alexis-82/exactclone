<script lang="ts">
  import { t, lang, errorParts } from '../i18n/i18n'
  import { DestinationInfo, PickArchive, PickFolder, ReadArchiveInfo, StartRestore } from '../../wailsjs/go/main/App'
  import type { archive, mount } from '../../wailsjs/go/models'
  import { formatBytes } from './format'
  import { jobStarted } from './job'
  import ConfirmDialog from './ConfirmDialog.svelte'

  let archivePath = $state('')
  let manifest = $state<archive.Manifest | null>(null)
  let destDir = $state('')
  let destInfo = $state<mount.Info | null>(null)
  let error = $state<{ message: string; detail: string } | null>(null)
  let confirming = $state(false)

  // Apparent size of the archived files (sparse files are restored in full);
  // archives without it fall back to the used space of the partitions.
  const needed = $derived(
    manifest?.contentBytes || (manifest?.partitions ?? []).reduce((sum, p) => sum + p.usedBytes, 0),
  )
  // Restoring into a folder needs no admin rights (the backend does not require them).
  const canStart = $derived(!!manifest && !!destDir && !!destInfo && destInfo.freeBytes >= needed)

  async function chooseArchive() {
    error = null
    try {
      const p = await PickArchive()
      if (!p) return
      manifest = null
      archivePath = p
      manifest = await ReadArchiveInfo(p)
    } catch (e) {
      error = errorParts($lang, e)
    }
  }

  async function chooseFolder() {
    error = null
    try {
      const p = await PickFolder()
      if (!p) return
      destDir = p
      destInfo = await DestinationInfo(p)
    } catch (e) {
      error = errorParts($lang, e)
    }
  }

  async function start() {
    confirming = false
    error = null
    try {
      await StartRestore(archivePath, destDir)
      jobStarted('restore')
    } catch (e) {
      error = errorParts($lang, e)
    }
  }
</script>

<section>
  <div class="field">
    <span class="label">{$t('restore.archive')}</span>
    <div class="row">
      <input type="text" readonly value={archivePath} />
      <button onclick={chooseArchive}>{$t('restore.choose')}</button>
    </div>
    {#if manifest}
      <p class="muted">
        {$t('restore.created', { date: new Date(manifest.createdAt).toLocaleString($lang), disk: manifest.sourceDisk })}
      </p>
      <p class="muted">
        {$t('restore.partitions', { names: manifest.partitions.map((p) => `${p.name} (${p.fsType})`).join(', ') })}
      </p>
    {/if}
  </div>

  <div class="field">
    <span class="label">{$t('restore.destination')}</span>
    <div class="row">
      <input type="text" readonly value={destDir} />
      <button onclick={chooseFolder}>{$t('restore.choose')}</button>
    </div>
    {#if destInfo}
      <p class="muted">{$t('backup.free', { free: formatBytes(destInfo.freeBytes), fs: destInfo.fsType || '?' })}</p>
    {/if}
    {#if manifest}
      <p class:danger={destInfo && destInfo.freeBytes < needed}>{$t('restore.needed', { size: formatBytes(needed) })}</p>
    {/if}
  </div>

  {#if error}
    <div class="error" role="alert">
      {error.message}
      {#if error.detail}<div class="detail">{error.detail}</div>{/if}
    </div>
  {/if}

  <div class="actions">
    <button class="primary" disabled={!canStart} onclick={() => (confirming = true)}>{$t('restore.start')}</button>
  </div>
</section>

{#if confirming}
  <ConfirmDialog
    rows={[
      { label: $t('confirm.source'), value: archivePath },
      { label: $t('confirm.destination'), value: destDir },
      { label: $t('confirm.size'), value: formatBytes(needed) },
    ]}
    onconfirm={start}
    oncancel={() => (confirming = false)}
  />
{/if}
