<script lang="ts">
  import { onMount } from 'svelte'
  import { t, lang, errorParts } from '../i18n/i18n'
  import { ListDisks, ListFileSystems, StartFormat } from '../../wailsjs/go/main/App'
  import type { disk, format } from '../../wailsjs/go/models'
  import { formatBytes } from './format'
  import { diskLabel, formatReason } from './disks'
  import { jobStarted } from './job'
  import ConfirmDialog from './ConfirmDialog.svelte'

  let { elevated, devSafe }: { elevated: boolean; devSafe: boolean } = $props()

  const MAX_MBR = 2 ** 41 // format.MaxMBRBytes
  const FAT_FORBIDDEN = '*?.,;:/\\|+=<>[]"'

  let disks = $state<disk.Disk[]>([])
  let fileSystems = $state<format.FS[]>([])
  let dstId = $state('')
  let fsId = $state('')
  let scheme = $state<'mbr' | 'gpt'>('mbr')
  let label = $state('')
  let full = $state(false)
  let error = $state<{ message: string; detail: string } | null>(null)
  let confirming = $state(false)

  const dst = $derived(disks.find((d) => d.id === dstId))
  const fs = $derived(fileSystems.find((f) => f.id === fsId))
  const diskName = (d: disk.Disk) => diskLabel($t, d)
  const dstReason = (d: disk.Disk) => formatReason($t, d, devSafe)
  const mbrReason = $derived(dst && dst.sizeBytes > MAX_MBR ? $t('format.reasonMbr') : '')
  const labelError = $derived(fs ? checkLabel(fs, label) : '')
  const canStart = $derived(elevated && !!dst && !!fs && !dstReason(dst) && !fsReason(fs) && !labelError)

  /** Why f cannot be used on the selected disk ('' if it can). Mirrors validate.Format. */
  function fsReason(f: format.FS): string {
    if (!f.available) return $t('format.reasonTool', { tool: f.tool })
    if (!dst) return ''
    if (dst.sizeBytes < f.minBytes) return $t('format.reasonTooSmall', { size: formatBytes(f.minBytes) })
    if (f.maxBytes > 0 && dst.sizeBytes > f.maxBytes) return $t('format.reasonTooLarge', { size: formatBytes(f.maxBytes) })
    return ''
  }

  /** Mirrors format.CheckLabel. */
  function checkLabel(f: format.FS, l: string): string {
    const n = f.labelBytes ? new TextEncoder().encode(l).length : [...l].length
    if (n > f.labelMax) return $t(f.labelBytes ? 'format.labelTooLongBytes' : 'format.labelTooLong', { max: f.labelMax })
    const fat = f.id === 'fat32' || f.id === 'exfat'
    for (const c of l) {
      if (c < ' ' || c === '\u007f' || c === '"' || (fat && FAT_FORBIDDEN.includes(c))) {
        return $t('format.labelInvalid', { char: c < ' ' ? '\\n' : c })
      }
    }
    return ''
  }

  /** Keeps the selections valid after the disk or the lists change. */
  function adjust() {
    if (dst && dstReason(dst)) dstId = ''
    if (!fs || fsReason(fs)) fsId = fileSystems.find((f) => !fsReason(f))?.id ?? ''
    if (mbrReason) scheme = 'gpt'
  }

  async function refresh() {
    error = null
    try {
      ;[disks, fileSystems] = await Promise.all([ListDisks(), ListFileSystems()])
      disks ??= []
      fileSystems ??= []
    } catch (e) {
      error = errorParts($lang, e)
      disks = []
    }
    adjust()
  }

  async function start() {
    confirming = false
    error = null
    try {
      await StartFormat(dstId, fsId, scheme, label, full)
      jobStarted('format')
    } catch (e) {
      error = errorParts($lang, e)
    }
  }

  onMount(refresh)
</script>

<section>
  <div class="field">
    <label for="fdst">{$t('format.destination')}</label>
    <div class="row">
      <select id="fdst" bind:value={dstId} onchange={adjust}>
        <option value="">{$t('backup.selectDisk')}</option>
        {#each disks as d (d.id)}
          <option value={d.id} disabled={!!dstReason(d)}>
            {diskName(d)}{dstReason(d) ? ` — ${dstReason(d)}` : ''}
          </option>
        {/each}
      </select>
      <button onclick={refresh}>{$t('backup.refresh')}</button>
    </div>
  </div>

  <div class="field">
    <label for="ffs">{$t('format.fs')}</label>
    <select id="ffs" bind:value={fsId}>
      {#each fileSystems as f (f.id)}
        <option value={f.id} disabled={!!fsReason(f)}>
          {f.name}{fsReason(f) ? ` — ${fsReason(f)}` : ''}
        </option>
      {/each}
    </select>
    <p class="muted">{$t('format.singlePartition')}</p>
  </div>

  <div class="field">
    <label for="fscheme">{$t('format.scheme')}</label>
    <select id="fscheme" bind:value={scheme}>
      <option value="mbr" disabled={!!mbrReason}>{$t('format.schemeMbr')}{mbrReason ? ` — ${mbrReason}` : ''}</option>
      <option value="gpt">{$t('format.schemeGpt')}</option>
    </select>
  </div>

  <div class="field">
    <label for="flabel">{$t('format.label')}</label>
    <input id="flabel" type="text" bind:value={label} placeholder={$t('format.labelPlaceholder')} />
    {#if labelError}<p class="danger">{labelError}</p>{/if}
  </div>

  <label class="check">
    <input type="checkbox" bind:checked={full} />
    <span>
      {$t('format.full')}
      <span class="muted">— {$t('format.fullHint')}</span>
    </span>
  </label>

  {#if error}
    <div class="error" role="alert">
      {error.message}
      {#if error.detail}<div class="detail">{error.detail}</div>{/if}
    </div>
  {/if}

  <div class="actions">
    <button class="primary" disabled={!canStart} onclick={() => (confirming = true)}>{$t('format.start')}</button>
  </div>
</section>

{#if confirming && dst && fs}
  <ConfirmDialog
    rows={[
      { label: $t('confirm.destination'), value: diskName(dst) },
      { label: $t('confirm.fs'), value: fs.name },
      { label: $t('confirm.scheme'), value: scheme.toUpperCase() },
      { label: $t('confirm.label'), value: label || $t('confirm.none') },
      { label: $t('confirm.mode'), value: full ? $t('confirm.modeFull') : $t('confirm.modeQuick') },
    ]}
    warning={$t('confirm.formatWarning', { disk: diskName(dst) })}
    requireAck
    onconfirm={start}
    oncancel={() => (confirming = false)}
  />
{/if}
