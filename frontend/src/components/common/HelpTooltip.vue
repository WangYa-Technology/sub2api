<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, useTemplateRef, nextTick } from 'vue'
import { getTooltipPosition } from '@/utils/floatingPanel'

const props = withDefaults(defineProps<{
  content?: string
  trigger?: 'hover' | 'click'
  widthClass?: string
  tooltipClass?: string
  openDelayMs?: number
}>(), {
  trigger: 'hover',
  widthClass: 'w-64',
  openDelayMs: 0,
})

const show = ref(false)
const triggerRef = useTemplateRef<HTMLElement>('trigger')
const tooltipRef = useTemplateRef<HTMLElement>('tooltip')
// 上方放不下时翻到图标下方；箭头始终对准图标中心
const placeBelow = ref(false)
const arrowLeft = ref('50%')
const tooltipStyle = ref({ top: '0px', left: '0px', maxHeight: 'none' })
let openTimer: ReturnType<typeof setTimeout> | null = null

function clearOpenTimer() {
  if (openTimer == null) return
  clearTimeout(openTimer)
  openTimer = null
}

function showTooltip() {
  openTimer = null
  show.value = true
  nextTick(updatePosition)
}

function openTooltip(immediate = false) {
  clearOpenTimer()
  const delay = immediate ? 0 : Math.max(0, props.openDelayMs)
  if (delay === 0) {
    showTooltip()
    return
  }
  openTimer = setTimeout(showTooltip, delay)
}

function closeTooltip() {
  clearOpenTimer()
  show.value = false
}

function onEnter() {
  if (props.trigger !== 'hover') return
  openTooltip()
}

function onFocusIn() {
  if (props.trigger !== 'hover') return
  openTooltip(true)
}

function isInside(container: HTMLElement | null, target: EventTarget | null): boolean {
  return target instanceof Node && !!container?.contains(target)
}

// 悬停模式下指针在触发图标与提示框之间往返时保持打开，便于选中提示里的文字。
function onLeave(event: MouseEvent | FocusEvent) {
  if (props.trigger !== 'hover') return
  if (isInside(tooltipRef.value, event.relatedTarget)) return
  closeTooltip()
}

function onTooltipLeave(event: MouseEvent) {
  if (props.trigger !== 'hover') return
  if (isInside(triggerRef.value, event.relatedTarget)) return
  closeTooltip()
}

function onClick(event: MouseEvent) {
  if (props.trigger !== 'click') return
  event.stopPropagation()
  if (show.value) {
    closeTooltip()
    return
  }
  openTooltip(true)
}

function onDocumentClick(event: MouseEvent) {
  if (props.trigger !== 'click' || !show.value) return
  const target = event.target as Node | null
  if (!target) return
  if (triggerRef.value?.contains(target) || tooltipRef.value?.contains(target)) return
  closeTooltip()
}

function onDocumentKeydown(event: KeyboardEvent) {
  if (props.trigger !== 'click') return
  if (event.key === 'Escape') {
    closeTooltip()
  }
}

function onViewportChange() {
  if (!show.value) return
  updatePosition()
}

function updatePosition() {
  const el = triggerRef.value
  const tip = tooltipRef.value
  if (!el || !tip) return
  const rect = el.getBoundingClientRect()
  const { width, height } = tip.getBoundingClientRect()
  const viewportWidth = document.documentElement.clientWidth || window.innerWidth
  const viewportHeight = window.innerHeight
  const viewportPadding = 8
  const gap = 8
  const spaceAbove = Math.max(0, rect.top - gap - viewportPadding)
  const spaceBelow = Math.max(0, viewportHeight - rect.bottom - gap - viewportPadding)
  // Measure the full content even after a previous viewport constrained it.
  // Bound tall content before positioning so the tooltip can still flip below
  // a trigger near the top instead of being clamped over the trigger.
  const contentHeight = Math.max(height, tip.scrollHeight)
  const placeAbove = contentHeight <= spaceAbove || (contentHeight > spaceBelow && spaceAbove >= spaceBelow)
  const maxHeight = placeAbove ? spaceAbove : spaceBelow
  const { left, top } = getTooltipPosition(
    rect,
    { width, height: Math.min(contentHeight, maxHeight) },
    viewportWidth,
    viewportHeight
  )
  placeBelow.value = top >= rect.bottom
  arrowLeft.value = `${Math.min(Math.max(rect.left + rect.width / 2 - left, 8), Math.max(8, width - 8))}px`
  tooltipStyle.value = {
    top: `${top}px`,
    left: `${left}px`,
    maxHeight: `${maxHeight}px`,
  }
}

onMounted(() => {
  document.addEventListener('click', onDocumentClick, true)
  document.addEventListener('keydown', onDocumentKeydown)
  window.addEventListener('resize', onViewportChange)
  window.addEventListener('scroll', onViewportChange, true)
})

onBeforeUnmount(() => {
  clearOpenTimer()
  document.removeEventListener('click', onDocumentClick, true)
  document.removeEventListener('keydown', onDocumentKeydown)
  window.removeEventListener('resize', onViewportChange)
  window.removeEventListener('scroll', onViewportChange, true)
})
</script>

<template>
  <div
    ref="trigger"
    class="group relative ml-1 inline-flex items-center align-middle"
    @mouseenter="onEnter"
    @mouseleave="onLeave"
    @focusin="onFocusIn"
    @focusout="onLeave"
    @click="onClick"
  >
    <!-- Trigger Icon -->
    <slot name="trigger">
      <svg
        class="h-4 w-4 cursor-help text-gray-400 transition-colors hover:text-primary-600 dark:text-gray-500 dark:hover:text-primary-400"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        stroke-width="2"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
        />
      </svg>
    </slot>

    <!-- Teleport to body to escape modal overflow clipping -->
    <Teleport to="body">
      <!-- before: 伪元素朝触发图标延伸一段透明区域，盖住提示框与图标之间的空隙，让指针能连续移入提示框。 -->
      <div
        ref="tooltip"
        v-show="show"
        role="tooltip"
        :class="[
          'fixed z-[99999] overflow-y-auto rounded-lg bg-gray-900 p-3 text-xs leading-relaxed text-white shadow-xl ring-1 ring-white/10 selection:bg-primary-200 selection:text-gray-900 before:absolute before:inset-x-0 before:h-3 dark:bg-gray-800 dark:selection:bg-primary-200 dark:selection:text-gray-900',
          placeBelow ? 'before:bottom-full' : 'before:top-full',
          props.widthClass,
          props.tooltipClass,
        ]"
        :style="tooltipStyle"
        @mouseleave="onTooltipLeave"
      >
        <button
          v-if="props.trigger === 'click'"
          type="button"
          class="absolute right-1.5 top-1.5 rounded p-1 text-gray-300 transition-colors hover:bg-white/10 hover:text-white"
          aria-label="Close"
          @click.stop="closeTooltip"
        >
          <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
        <slot>{{ content }}</slot>
        <div
          :class="['absolute h-2 w-2 -translate-x-1/2 rotate-45 bg-gray-900 dark:bg-gray-800', placeBelow ? '-top-1' : '-bottom-1']"
          :style="{ left: arrowLeft }"
        ></div>
      </div>
    </Teleport>
  </div>
</template>
