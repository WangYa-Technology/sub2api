<template>
  <div ref="homeRoot" class="min-h-screen bg-white" v-html="homeHtml"></div>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'

const homeRoot = ref<HTMLElement | null>(null)
const homeHtml = ref('')

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

    const response = await fetch('/hcai/page.html', { cache: 'no-cache' })
    const html = await response.text()
    const documentFragment = new DOMParser().parseFromString(html, 'text/html')
    const page = documentFragment.querySelector<HTMLElement>('.hc-page')

    if (!page) {
      return
    }

    rewriteStaticAssetPaths(page)
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
