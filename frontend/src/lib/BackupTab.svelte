<script lang="ts">
  import { onMount } from 'svelte'
  import { t, lang, errorParts } from '../i18n/i18n'
  import {
    DestinationInfo,
    EstimateArchive,
    ListDisks,
    PickSaveFile,
    StartArchive,
    StartClone,
  } from '../../wailsjs/go/main/App'
  import type { disk, mount } from '../../wailsjs/go/models'
  import { dirname, formatBytes } from './format'
  import { jobStarted } from './job'
  import ConfirmDialog from './ConfirmDialog.svelte'

  let { elevated, devSafe }: { elevated: boolean; devSafe: boolean } = $props()

  const FAT32_MAX = 4294967295
  const safeBuses = ['usb', 'loop', 'virtual']

  let mode = $state<'disk' | 'file'>('disk')
  let disks = $state<disk.Disk[]>([])
  let srcId = $state('')
  let dstId = $state('')
  let verify = $state(true)
  let selected = $state<string[]>([])
  let outPath = $state('')
  let destInfo = $state<mount.Info | null>(null)
  let estimate = $state<number | null>(null)
  let estimating = $state(false)
  let error = $state<{ message: string; detail: string } | null>(null)
  let confirming = $state(false)
  let estimateToken = 0

  const src = $derived(disks.find((d) => d.id === srcId))
  const dst = $derived(disks.find((d) => d.id === dstId))

  function label(d: disk.Disk): string {
    const sys = d.isSystem ? ` [${$t('disk.system')}]` : ''
    return `${d.model || d.id} — ${formatBytes(d.sizeBytes)} · ${d.bus} · ${d.id}${sys}`
  }

  function srcReason(d: disk.Disk): string {
    return d.isSystem ? $t('disk.reasonSystemSource') : ''
  }

  function dstReason(d: disk.Disk): string {
    if (!src) return ''
    if (d.id === src.id) return $t('disk.reasonSource')
    if (d.isSystem) return $t('disk.reasonSystem')
    if (d.sizeBytes < src.sizeBytes) return $t('disk.reasonTooSmall')
    if (devSafe && !safeBuses.includes(d.bus)) return $t('disk.reasonDevSafe')
    return ''
  }

  function showError(e: unknown) {
    error = errorParts($lang, e)
  }

  async function refresh() {
    error = null
    try {
      disks = (await ListDisks()) ?? []
    } catch (e) {
      showError(e)
      disks = []
    }
    if (!src || srcReason(src)) srcId = ''
    if (!dst || dstReason(dst)) dstId = ''
    onSourceChange()
  }

  function onSourceChange() {
    if (dst && dstReason(dst)) dstId = ''
    selected = src ? src.partitions.filter((p) => p.supported).map((p) => p.id) : []
    updateEstimate()
  }

  async function updateEstimate() {
    const token = ++estimateToken
    estimate = null
    if (mode !== 'file' || !src || selected.length === 0) return
    estimating = true
    try {
      const est = await EstimateArchive(src.id, selected)
      if (token === estimateToken) estimate = est.totalBytes
    } catch (e) {
      if (token === estimateToken) showError(e)
    } finally {
      if (token === estimateToken) estimating = false
    }
  }

  function togglePartition(id: string, on: boolean) {
    selected = on ? [...selected, id] : selected.filter((x) => x !== id)
    updateEstimate()
  }

  function defaultArchiveName(): string {
    const d = new Date()
    const date = `${d.getFullYear()}${String(d.getMonth() + 1).padStart(2, '0')}${String(d.getDate()).padStart(2, '0')}`
    const name = (src?.model || src?.id || 'disk').replace(/[^A-Za-z0-9_-]+/g, '_')
    return `backup_${name}_${date}.tar.zst`
  }

  async function chooseOutput() {
    error = null
    try {
      let p = await PickSaveFile(defaultArchiveName())
      if (!p) return
      if (!p.toLowerCase().endsWith('.tar.zst')) p += '.tar.zst'
      outPath = p
      destInfo = await DestinationInfo(dirname(p))
    } catch (e) {
      showError(e)
    }
  }

  const fat32TooBig = $derived(destInfo?.fsType === 'vfat' && estimate !== null && estimate > FAT32_MAX)

  const canStart = $derived.by(() => {
    if (!elevated || !src || srcReason(src)) return false
    if (mode === 'disk') return !!dst && !dstReason(dst)
    return (
      selected.length > 0 &&
      !!outPath &&
      !!destInfo &&
      estimate !== null &&
      !estimating &&
      destInfo.freeBytes >= estimate &&
      !fat32TooBig
    )
  })

  const confirmRows = $derived.by(() => {
    if (!src) return []
    if (mode === 'disk' && dst) {
      return [
        { label: $t('confirm.source'), value: label(src) },
        { label: $t('confirm.destination'), value: label(dst) },
        { label: $t('confirm.size'), value: formatBytes(src.sizeBytes) },
        { label: $t('confirm.verify'), value: verify ? $t('confirm.yes') : $t('confirm.no') },
      ]
    }
    const names = src.partitions.filter((p) => selected.includes(p.id)).map((p) => p.label || p.id)
    return [
      { label: $t('confirm.source'), value: `${label(src)}\n${names.join(', ')}` },
      { label: $t('confirm.destination'), value: outPath },
      { label: $t('confirm.size'), value: formatBytes(estimate ?? 0) },
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
        await StartArchive(srcId, selected, outPath)
        jobStarted('archive')
      }
    } catch (e) {
      showError(e)
    }
  }

  onMount(refresh)
</script>

<section>
  <fieldset>
    <legend>{$t('backup.type')}</legend>
    <label class="choice">
      <input type="radio" bind:group={mode} value="disk" onchange={updateEstimate} />
      <span><strong>{$t('backup.typeDisk')}</strong><br /><small>{$t('backup.typeDiskHint')}</small></span>
    </label>
    <label class="choice">
      <input type="radio" bind:group={mode} value="file" onchange={updateEstimate} />
      <span><strong>{$t('backup.typeFile')}</strong><br /><small>{$t('backup.typeFileHint')}</small></span>
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
    <label class="check">
      <input type="checkbox" bind:checked={verify} />
      {$t('backup.verify')}
    </label>
  {:else}
    <div class="field">
      <span class="label">{$t('backup.partitions')}</span>
      {#if src && src.partitions.length === 0}
        <p class="muted">{$t('backup.noPartitions')}</p>
      {/if}
      {#each src?.partitions ?? [] as p (p.id)}
        <label class="check" class:muted={!p.supported}>
          <input
            type="checkbox"
            disabled={!p.supported}
            checked={selected.includes(p.id)}
            onchange={(e) => togglePartition(p.id, e.currentTarget.checked)}
          />
          {p.label || p.id} — {p.fsType || '?'} — {formatBytes(p.sizeBytes)}
          {#if p.mountPoints?.length}<small>({p.mountPoints.join(', ')})</small>{/if}
          {#if !p.supported}<small>({$t('backup.unsupported')})</small>{/if}
        </label>
      {/each}
    </div>
    <div class="field">
      <span class="label">{$t('backup.archiveFile')}</span>
      <div class="row">
        <input type="text" readonly value={outPath} />
        <button onclick={chooseOutput} disabled={!src}>{$t('backup.choose')}</button>
      </div>
      {#if destInfo}
        <p class="muted">{$t('backup.free', { free: formatBytes(destInfo.freeBytes), fs: destInfo.fsType || '?' })}</p>
      {/if}
      {#if estimating}
        <p class="muted">{$t('backup.estimating')}</p>
      {:else if estimate !== null}
        <p class:danger={destInfo && destInfo.freeBytes < estimate}>{$t('backup.estimate', { size: formatBytes(estimate) })}</p>
      {/if}
      {#if destInfo?.fsType === 'vfat'}
        <p class:danger={fat32TooBig} class:warn={!fat32TooBig}>{$t('backup.fat32Warning')}</p>
      {/if}
    </div>
  {/if}

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
