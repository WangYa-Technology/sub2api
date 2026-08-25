<template>
  <div ref="homeRoot" class="min-h-screen bg-white dark:bg-[#080c0c]" v-html="homeHtml"></div>

  <Teleport v-if="themeToggleReady" to="[data-home-theme-slot]">
    <button
      type="button"
      class="hc-theme-toggle"
      :aria-label="themeToggleLabel"
      :aria-pressed="isDark"
      :title="themeToggleLabel"
      data-home-theme-toggle
      @click="toggleTheme"
    >
      <Icon :name="isDark ? 'sun' : 'moon'" size="sm" :stroke-width="1.8" aria-hidden="true" />
    </button>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import {
  isDarkTheme,
  setTheme,
  THEME_CHANGE_EVENT,
  type ThemeChangeDetail
} from '@/utils/theme'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

const homeRoot = ref<HTMLElement | null>(null)
const homeHtml = ref('')
const isDark = ref(isDarkTheme())
const themeToggleReady = ref(false)
const appStore = useAppStore()
const themeToggleLabel = computed(() => isDark.value ? '切换至亮色模式' : '切换至暗色模式')

let stylesheetElement: HTMLLinkElement | null = null
let scriptElement: HTMLScriptElement | null = null

function toggleTheme() {
  setTheme(isDark.value ? 'light' : 'dark')
}

function handleThemeChange(event: Event) {
  const detail = (event as CustomEvent<ThemeChangeDetail>).detail
  isDark.value = detail.theme === 'dark'
}

function ensureStylesheet() {
  if (document.querySelector('link[data-hcai-home-style]')) {
    return
  }

  stylesheetElement = document.createElement('link')
  stylesheetElement.rel = 'stylesheet'
  stylesheetElement.href = '/hcai/style.css?v=home-theme-v1'
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
  document.addEventListener(THEME_CHANGE_EVENT, handleThemeChange)

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
    themeToggleReady.value = true
    runPageScript()
  })()
})

onBeforeUnmount(() => {
  document.removeEventListener(THEME_CHANGE_EVENT, handleThemeChange)
  themeToggleReady.value = false
  scriptElement?.remove()
  scriptElement = null
})
</script>
