<script lang="ts">
  import { onMount } from 'svelte'
  import { t, lang, errorParts } from '../i18n/i18n'
  import { DestinationInfo, ListDisks, PickSaveFile, StartClone, StartImage } from '../../wailsjs/go/main/App'
  import type { disk, mount } from '../../wailsjs/go/models'
  import { dirname, formatBytes } from './format'
  import { destinationReason, diskLabel, sourceReason } from './disks'
  import { jobStarted } from './job'
  import ConfirmDialog from './ConfirmDialog.svelte'

  let { elevated, devSafe }: { elevated: boolean; devSafe: boolean } = $props()

  const FAT32_MAX = 4294967295

  let mode = $state<'disk' | 'image'>('disk')
  let disks = $state<disk.Disk[]>([])
  let srcId = $state('')
  let dstId = $state('')
  let verify = $state(true)
  let outPath = $state('')
  let destInfo = $state<mount.Info | null>(null)
  let error = $state<{ message: string; detail: string } | null>(null)
  let confirming = $state(false)

  const src = $derived(disks.find((d) => d.id === srcId))
  const dst = $derived(disks.find((d) => d.id === dstId))

  const label = (d: disk.Disk) => diskLabel($t, d)
  const srcReason = (d: disk.Disk) => sourceReason($t, d)
  const dstReason = (d: disk.Disk) => (src ? destinationReason($t, d, src.sizeBytes, src.id, devSafe) : '')

  async function refresh() {
    error = null
    try {
      disks = (await ListDisks()) ?? []
    } catch (e) {
      error = errorParts($lang, e)
      disks = []
    }
    if (!src || srcReason(src)) srcId = ''
    if (!dst || dstReason(dst)) dstId = ''
  }

  function onSourceChange() {
    if (dst && dstReason(dst)) dstId = ''
  }

  function defaultImageName(): string {
    const d = new Date()
    const date = `${d.getFullYear()}${String(d.getMonth() + 1).padStart(2, '0')}${String(d.getDate()).padStart(2, '0')}`
    const name = (src?.model || src?.id || 'disk').replace(/[^A-Za-z0-9_-]+/g, '_')
    return `${name}_${date}.img.zst`
  }

  async function chooseOutput() {
    error = null
    try {
      let p = await PickSaveFile(defaultImageName())
      if (!p) return
      if (!p.toLowerCase().endsWith('.img.zst')) p += '.img.zst'
      outPath = p
      destInfo = await DestinationInfo(dirname(p))
    } catch (e) {
      error = errorParts($lang, e)
    }
  }

  const fat32TooBig = $derived(destInfo?.fsType === 'vfat' && !!src && src.sizeBytes > FAT32_MAX)
  const spaceOk = $derived(!!destInfo && !!src && destInfo.freeBytes >= src.sizeBytes)

  const canStart = $derived.by(() => {
    if (!elevated || !src || srcReason(src)) return false
    if (mode === 'disk') return !!dst && !dstReason(dst)
    return !!outPath && spaceOk && !fat32TooBig
  })

  const confirmRows = $derived.by(() => {
    if (!src) return []
    return [
      { label: $t('confirm.source'), value: label(src) },
      { label: $t('confirm.destination'), value: mode === 'disk' && dst ? label(dst) : outPath },
      { label: $t('confirm.size'), value: formatBytes(src.sizeBytes) },
      { label: $t('confirm.verify'), value: verify ? $t('confirm.yes') : $t('confirm.no') },
    ]
  })

  async function start() {
    confirming = false
    error = null
    try {
      if (mode === 'disk') {
        await StartClone(srcId, dstId, verify)
        jobStarted('clone')
      } else {
        await StartImage(srcId, outPath, verify)
        jobStarted('image')
      }
    } catch (e) {
      error = errorParts($lang, e)
    }
  }

  onMount(refresh)
</script>

<section>
  <fieldset>
    <legend>{$t('backup.type')}</legend>
    <label class="choice">
      <input type="radio" bind:group={mode} value="disk" />
      <span><strong>{$t('backup.typeDisk')}</strong><br /><small>{$t('backup.typeDiskHint')}</small></span>
    </label>
    <label class="choice">
      <input type="radio" bind:group={mode} value="image" />
      <span><strong>{$t('backup.typeImage')}</strong><br /><small>{$t('backup.typeImageHint')}</small></span>
    </label>
  </fieldset>

  <div class="field">
    <label for="src">{$t('backup.source')}</label>
    <div class="row">
      <select id="src" bind:value={srcId} onchange={onSourceChange}>
        <option value="">{$t('backup.selectDisk')}</option>
        {#each disks as d (d.id)}
          <option value={d.id} disabled={!!srcReason(d)}>
            {label(d)}{srcReason(d) ? ` — ${srcReason(d)}` : ''}
          </option>
        {/each}
      </select>
      <button onclick={refresh}>{$t('backup.refresh')}</button>
    </div>
    {#if disks.length === 0}<p class="muted">{$t('backup.noDisks')}</p>{/if}
  </div>

  {#if mode === 'disk'}
    <div class="field">
      <label for="dst">{$t('backup.destination')}</label>
      <select id="dst" bind:value={dstId} disabled={!src}>
        <option value="">{$t('backup.selectDisk')}</option>
        {#each disks as d (d.id)}
          <option value={d.id} disabled={!!dstReason(d)}>
            {label(d)}{dstReason(d) ? ` — ${dstReason(d)}` : ''}
          </option>
        {/each}
      </select>
    </div>
  {:else}
    <div class="field">
      <span class="label">{$t('backup.imageFile')}</span>
      <div class="row">
        <input type="text" readonly value={outPath} />
        <button onclick={chooseOutput} disabled={!src}>{$t('backup.choose')}</button>
      </div>
      {#if destInfo && src}
        <p class:danger={!spaceOk}>
          {$t('backup.space', {
            free: formatBytes(destInfo.freeBytes),
            fs: destInfo.fsType || '?',
            need: formatBytes(src.sizeBytes),
          })}
        </p>
        <p class="muted"><small>{$t('backup.spaceHint')}</small></p>
      {/if}
      {#if fat32TooBig}
        <p class="danger">{$t('backup.fat32Warning')}</p>
      {/if}
    </div>
  {/if}

  <label class="check">
    <input type="checkbox" bind:checked={verify} />
    {$t('backup.verify')}
  </label>

  {#if error}
    <div class="error" role="alert">
      {error.message}
      {#if error.detail}<div class="detail">{error.detail}</div>{/if}
    </div>
  {/if}

  <div class="actions">
    <button class="primary" disabled={!canStart} onclick={() => (confirming = true)}>{$t('backup.start')}</button>
  </div>
</section>

{#if confirming}
  <ConfirmDialog
    rows={confirmRows}
    warning={mode === 'disk' && dst ? $t('confirm.cloneWarning', { disk: label(dst) }) : ''}
    requireAck={mode === 'disk'}
    onconfirm={start}
    oncancel={() => (confirming = false)}
  />
{/if}
