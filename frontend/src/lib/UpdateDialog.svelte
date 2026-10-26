<script lang="ts">
  import { t, lang, errorParts } from '../i18n/i18n'
  import { InstallUpdate } from '../../wailsjs/go/main/App'
  import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'
  import { isSafeUrl, renderMarkdown } from './markdown'
  import { updateOffer, updateProgress } from './update'

  let installing = $state(false)
  let error = $state<{ message: string; detail: string } | null>(null)

  const percent = $derived(
    $updateProgress && $updateProgress.total > 0 ? Math.floor(($updateProgress.done / $updateProgress.total) * 100) : 0,
  )

  function close() {
    if (installing) return
    updateOffer.set(null)
    error = null
  }

  // On success the backend starts the installer and quits the app.
  async function install() {
    error = null
    installing = true
    updateProgress.set(null)
    try {
      await InstallUpdate()
    } catch (e) {
      error = errorParts($lang, e)
      installing = false
    }
  }

  function open(url: string) {
    if (isSafeUrl(url)) BrowserOpenURL(url)
  }

  // Release notes links open in the system browser, never inside the app.
  function onNotesClick(e: MouseEvent) {
    const a = (e.target as HTMLElement).closest('a')
    if (!a) return
    e.preventDefault()
    open(a.getAttribute('href') ?? '')
  }
</script>

{#if $updateOffer}
  {@const offer = $updateOffer}
  <div class="backdrop">
    <div class="dialog" role="dialog" aria-modal="true" aria-labelledby="update-title">
      <h2 id="update-title">{$t('update.title')}</h2>
      <p>{$t('update.available', { latest: offer.latest, current: offer.current })}</p>

      {#if offer.notes.trim()}
        <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
        <div class="notes" onclick={onNotesClick}>{@html renderMarkdown(offer.notes)}</div>
      {/if}

      {#if !offer.canInstall}
        <p class="muted">
          {$t('update.manual')}
          <button class="link" onclick={() => open(offer.pageUrl)}>{$t('update.releasePage')}</button>
        </p>
      {/if}

      {#if installing}
        <progress max="100" value={percent}></progress>
        <p class="muted">{percent < 100 ? $t('update.downloading', { percent }) : $t('update.starting')}</p>
      {/if}

      {#if error}
        <div class="error" role="alert">
          {error.message}
          {#if error.detail}<div class="detail">{error.detail}</div>{/if}
        </div>
      {/if}

      <div class="actions">
        <button onclick={close} disabled={installing}>{$t('update.later')}</button>
        {#if offer.canInstall}
          <button class="primary" onclick={install} disabled={installing}>{$t('update.install')}</button>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  p {
    line-height: 1.5;
  }
  .notes {
    max-height: 40vh;
    overflow: auto;
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 4px 12px;
    margin-bottom: 12px;
  }
  progress {
    width: 100%;
  }
  .error .detail {
    max-height: 120px;
    overflow: auto;
    white-space: pre-wrap;
  }
  button.link {
    border: none;
    background: none;
    padding: 0;
    color: var(--primary);
    text-decoration: underline;
    cursor: pointer;
  }
</style>
