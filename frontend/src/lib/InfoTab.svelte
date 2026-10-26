<script lang="ts">
  import { onMount } from 'svelte'
  import { t, lang, errorParts } from '../i18n/i18n'
  import { CheckUpdate, GetAppInfo, GetConfig, SetCheckUpdates } from '../../wailsjs/go/main/App'
  import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'
  import type { main } from '../../wailsjs/go/models'
  import { updateOffer } from './update'

  const SEVEN_ZIP_ZSTD = 'https://github.com/mcmilk/7-Zip-zstd/releases'

  let info = $state<main.AppInfo | null>(null)
  let checkAtStartup = $state(true)
  let checking = $state(false)
  let updateMessage = $state('')
  let updateError = $state<{ message: string; detail: string } | null>(null)

  onMount(async () => {
    info = await GetAppInfo()
    checkAtStartup = (await GetConfig()).checkUpdates
  })

  async function setCheckAtStartup(box: HTMLInputElement) {
    updateError = null
    try {
      await SetCheckUpdates(box.checked)
      checkAtStartup = box.checked
    } catch (e) {
      updateError = errorParts($lang, e)
      // not saved: show the setting that is still in effect
      box.checked = checkAtStartup
    }
  }

  async function checkUpdates() {
    updateMessage = ''
    updateError = null
    checking = true
    try {
      const u = await CheckUpdate()
      if (u.noRelease) updateMessage = $t('update.noRelease')
      else if (u.available) updateOffer.set(u)
      else if (u.current === 'dev') updateMessage = $t('update.devBuild', { latest: u.latest })
      else updateMessage = $t('update.upToDate', { current: u.current })
    } catch (e) {
      updateError = errorParts($lang, e)
    } finally {
      checking = false
    }
  }
</script>

<section>
  {#if info}
    <h2>{info.name} <span class="muted version">{$t('info.version', { version: info.version })}</span></h2>
    <p>{$t('info.description')}</p>

    <h3>{$t('update.section')}</h3>
    <label class="check">
      <input
        type="checkbox"
        checked={checkAtStartup}
        onchange={(e) => setCheckAtStartup(e.currentTarget)}
      />
      {$t('update.checkAtStartup')}
    </label>
    <p>
      <button onclick={checkUpdates} disabled={checking}>
        {checking ? $t('update.checking') : $t('update.checkNow')}
      </button>
    </p>
    {#if updateMessage}<p class="ok">{updateMessage}</p>{/if}
    {#if updateError}
      <div class="error" role="alert">
        {updateError.message}
        {#if updateError.detail}<div class="detail">{updateError.detail}</div>{/if}
      </div>
    {/if}

    <h3>{$t('info.system')}</h3>
    <dl>
      <dt>{$t('info.platform')}</dt>
      <dd>{info.os} / {info.arch}</dd>
      <dt>{$t('info.privileges')}</dt>
      <dd class:danger={!info.elevated}>{info.elevated ? $t('info.elevatedYes') : $t('info.elevatedNo')}</dd>
      <dt>{$t('info.devSafe')}</dt>
      <dd>{info.devSafe ? $t('info.on') : $t('info.off')}</dd>
      <dt>{$t('info.config')}</dt>
      <dd><code>{info.configPath}</code></dd>
    </dl>

    <h3>{$t('info.build')}</h3>
    <dl>
      <dt>Go</dt>
      <dd>{info.goVersion}</dd>
      <dt>Wails</dt>
      <dd>{info.wailsVersion || '—'}</dd>
      <dt>{$t('info.author')}</dt>
      <dd>{info.author}</dd>
    </dl>

    <h3>{$t('info.imageFormat')}</h3>
    <p>{$t('info.imageFormatText')}</p>
    <ul>
      <li>
        {$t('info.toolWindows')}
        <button class="link" onclick={() => BrowserOpenURL(SEVEN_ZIP_ZSTD)}>7-Zip-zstd</button>
      </li>
    </ul>
  {/if}
</section>

<style>
  h2 {
    margin-bottom: 4px;
  }
  .version {
    font-size: 14px;
    font-weight: normal;
  }
  h3 {
    font-size: 15px;
    margin: 20px 0 8px;
  }
  dl {
    display: grid;
    grid-template-columns: max-content 1fr;
    gap: 6px 16px;
    margin: 0;
  }
  dt {
    color: var(--muted);
  }
  dd {
    margin: 0;
    word-break: break-all;
  }
  ul {
    margin: 0;
    padding-left: 20px;
  }
  li {
    margin: 4px 0;
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
