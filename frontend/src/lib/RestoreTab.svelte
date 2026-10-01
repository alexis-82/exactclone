<script lang="ts">
  import { onMount } from 'svelte'
  import { t, lang, errorParts } from '../i18n/i18n'
  import { ListDisks, PickImage, ReadImageInfo, StartRestoreImage } from '../../wailsjs/go/main/App'
  import type { disk, image } from '../../wailsjs/go/models'
  import { formatBytes } from './format'
  import { destinationReason, diskLabel } from './disks'
  import { jobStarted } from './job'
  import ConfirmDialog from './ConfirmDialog.svelte'

  let { elevated, devSafe }: { elevated: boolean; devSafe: boolean } = $props()

  let imagePath = $state('')
  let info = $state<image.Info | null>(null)
  let disks = $state<disk.Disk[]>([])
  let dstId = $state('')
  let verify = $state(true)
  let error = $state<{ message: string; detail: string } | null>(null)
  let confirming = $state(false)

  const dst = $derived(disks.find((d) => d.id === dstId))
  const label = (d: disk.Disk) => diskLabel($t, d)
  const dstReason = (d: disk.Disk) => (info ? destinationReason($t, d, info.sizeBytes, '', devSafe) : '')
  const canStart = $derived(elevated && !!info && !!dst && !dstReason(dst))

  async function refresh() {
    error = null
    try {
      disks = (await ListDisks()) ?? []
    } catch (e) {
      error = errorParts($lang, e)
      disks = []
    }
    if (!dst || dstReason(dst)) dstId = ''
  }

  async function chooseImage() {
    error = null
    try {
      const p = await PickImage()
      if (!p) return
      info = null
      imagePath = p
      info = await ReadImageInfo(p)
      if (dst && dstReason(dst)) dstId = ''
    } catch (e) {
      error = errorParts($lang, e)
    }
  }

  async function start() {
    confirming = false
    error = null
    try {
      await StartRestoreImage(imagePath, dstId, verify)
      jobStarted('restore')
    } catch (e) {
      error = errorParts($lang, e)
    }
  }

  onMount(refresh)
</script>

<section>
  <div class="field">
    <span class="label">{$t('restore.image')}</span>
    <div class="row">
      <input type="text" readonly value={imagePath} />
      <button onclick={chooseImage}>{$t('restore.choose')}</button>
    </div>
    {#if info}
      <p class="muted">
        {$t('restore.info', {
          disk: info.sourceDisk,
          size: formatBytes(info.sizeBytes),
          date: new Date(info.createdAt).toLocaleString($lang),
        })}
      </p>
    {/if}
  </div>

  <div class="field">
    <label for="rdst">{$t('restore.destination')}</label>
    <div class="row">
      <select id="rdst" bind:value={dstId} disabled={!info}>
        <option value="">{$t('backup.selectDisk')}</option>
        {#each disks as d (d.id)}
          <option value={d.id} disabled={!!dstReason(d)}>
            {label(d)}{dstReason(d) ? ` — ${dstReason(d)}` : ''}
          </option>
        {/each}
      </select>
      <button onclick={refresh}>{$t('backup.refresh')}</button>
    </div>
  </div>

  <label class="check">
    <input type="checkbox" bind:checked={verify} />
    {$t('restore.verify')}
  </label>

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

{#if confirming && info && dst}
  <ConfirmDialog
    rows={[
      { label: $t('confirm.source'), value: `${imagePath}\n${info.sourceDisk}` },
      { label: $t('confirm.destination'), value: label(dst) },
      { label: $t('confirm.size'), value: formatBytes(info.sizeBytes) },
      { label: $t('confirm.verify'), value: verify ? $t('confirm.yes') : $t('confirm.no') },
    ]}
    warning={$t('confirm.cloneWarning', { disk: label(dst) })}
    requireAck
    onconfirm={start}
    oncancel={() => (confirming = false)}
  />
{/if}
