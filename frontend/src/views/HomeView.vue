<template>
  <div ref="homeRoot" class="min-h-screen bg-white" v-html="homeHtml"></div>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'

const homeRoot = ref<HTMLElement | null>(null)
const homeHtml = ref('')
const appStore = useAppStore()

let stylesheetElement: HTMLLinkElement | null = null
let scriptElement: HTMLScriptElement | null = null

function ensureStylesheet() {
  if (document.querySelector('link[data-hcai-home-style]')) {
    return
  }

  stylesheetElement = document.createElement('link')
  stylesheetElement.rel = 'stylesheet'
  stylesheetElement.href = '/hcai/style.css?v=gpt56-planets'
  stylesheetElement.dataset.hcaiHomeStyle = 'true'
  document.head.appendChild(stylesheetElement)
}

function rewriteStaticAssetPaths(container: HTMLElement) {
  container.querySelectorAll<HTMLElement>('[src], [href]').forEach((element) => {
    const attribute = element.hasAttribute('src') ? 'src' : 'href'
    const value = element.getAttribute(attribute)

    if (!value?.startsWith('./')) {
      return
    }

    element.setAttribute(attribute, `/hcai/${value.slice(2)}`)
  })
}

function addPublicModelPlazaEntry(container: HTMLElement) {
  const settings = appStore.cachedPublicSettings
  if (!settings?.model_plaza_enabled || settings.model_plaza_require_auth) {
    return
  }

  const navCta = container.querySelector<HTMLElement>('.hc-nav__cta')
  if (!navCta) {
    return
  }

  const entry = document.createElement('a')
  entry.href = '/model-plaza'
  entry.target = '_parent'
  entry.dataset.modelPlazaEntry = 'true'
  entry.textContent = '模型广场'
  navCta.before(entry)
}

function applyPublicDocUrl(container: HTMLElement) {
  const link = container.querySelector<HTMLAnchorElement>('[data-home-doc-link]')
  if (!link) {
    return
  }

  const docUrl = sanitizeUrl(appStore.cachedPublicSettings?.doc_url || '')
  if (!docUrl) {
    link.remove()
    return
  }

  link.href = docUrl
}

function runPageScript() {
  scriptElement?.remove()
  scriptElement = document.createElement('script')
  scriptElement.src = '/hcai/main.js?v=gpt56-planets'
  scriptElement.dataset.hcaiHomeScript = 'true'
  document.body.appendChild(scriptElement)
}

onMounted(() => {
  void (async () => {
    ensureStylesheet()

    if (!appStore.publicSettingsLoaded) {
      await appStore.fetchPublicSettings()
    }

    const response = await fetch('/hcai/page.html', { cache: 'no-cache' })
    const html = await response.text()
    const documentFragment = new DOMParser().parseFromString(html, 'text/html')
    const page = documentFragment.querySelector<HTMLElement>('.hc-page')

    if (!page) {
      return
    }

    rewriteStaticAssetPaths(page)
    applyPublicDocUrl(page)
    addPublicModelPlazaEntry(page)
    homeHtml.value = page.outerHTML

    await nextTick()
    runPageScript()
  })()
})

onBeforeUnmount(() => {
  scriptElement?.remove()
  scriptElement = null
})
</script>
