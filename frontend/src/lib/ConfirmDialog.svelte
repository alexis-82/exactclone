<script lang="ts">
  import { t } from '../i18n/i18n'

  let {
    rows,
    warning = '',
    requireAck = false,
    onconfirm,
    oncancel,
  }: {
    rows: { label: string; value: string }[]
    warning?: string
    requireAck?: boolean
    onconfirm: () => void
    oncancel: () => void
  } = $props()

  let ack = $state(false)
</script>

<div class="backdrop">
  <div class="dialog" role="dialog" aria-modal="true" aria-labelledby="confirm-title">
    <h2 id="confirm-title">{$t('confirm.title')}</h2>
    <dl>
      {#each rows as row}
        <dt>{row.label}</dt>
        <dd>{row.value}</dd>
      {/each}
    </dl>
    {#if warning}
      <p class="danger">{warning}</p>
    {/if}
    {#if requireAck}
      <label class="ack">
        <input type="checkbox" bind:checked={ack} />
        {$t('confirm.ack')}
      </label>
    {/if}
    <div class="actions">
      <button onclick={oncancel}>{$t('confirm.cancel')}</button>
      <button class="primary" class:danger-btn={requireAck} disabled={requireAck && !ack} onclick={onconfirm}>
        {$t('confirm.proceed')}
      </button>
    </div>
  </div>
</div>

<style>
  dl {
    display: grid;
    grid-template-columns: max-content 1fr;
    gap: 6px 16px;
    margin: 0 0 16px;
  }
  dt {
    color: var(--muted);
  }
  dd {
    margin: 0;
    word-break: break-all;
  }
  .ack {
    display: flex;
    gap: 8px;
    align-items: flex-start;
    margin: 12px 0;
  }
</style>
