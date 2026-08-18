<script lang="ts" setup>
// 分类选择组合框(纯手写, 无第三方依赖)。
// 替代原生 <input list=datalist>: datalist 弹出高度由浏览器锁死、CSS 不可控,
// 这里用 div 弹层实现, 高度可自由设定(约原生 2 倍), 同时保留"直接打字新建分类"的体验。
import {ref, onMounted, onBeforeUnmount} from 'vue'
import {t} from '../i18n'
import IconPark from './IconPark.vue'

const props = defineProps<{
  modelValue: string
  options: string[]
  placeholder?: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', v: string): void
  (e: 'enter'): void
}>()

const open = ref(false)
const rootEl = ref<HTMLElement | null>(null)

function onInput(e: Event) {
  emit('update:modelValue', (e.target as HTMLInputElement).value)
}

function toggle() {
  open.value = !open.value
}

function pick(v: string) {
  emit('update:modelValue', v)
  open.value = false
}

function onDocClick(e: MouseEvent) {
  if (rootEl.value && !rootEl.value.contains(e.target as Node)) {
    open.value = false
  }
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') open.value = false
  // 仅在自身 input 内按回车才触发 enter 事件(避免 textarea 等其它输入框的回车被误拦截)。
  if (e.key === 'Enter' && rootEl.value && rootEl.value.contains(e.target as Node)) {
    emit('enter')
  }
}

onMounted(() => {
  document.addEventListener('click', onDocClick)
  document.addEventListener('keydown', onKey)
})
onBeforeUnmount(() => {
  document.removeEventListener('click', onDocClick)
  document.removeEventListener('keydown', onKey)
})
</script>

<template>
  <div class="cat-select" ref="rootEl" :class="{open: open}">
    <div class="cat-select-wrap">
      <input
        class="form-input cat-select-input"
        :value="modelValue"
        :placeholder="placeholder || t('未分类')"
        @input="onInput"
        @focus="open = true"
      />
      <button type="button" class="cat-select-toggle" @click.stop="toggle" tabindex="-1"><IconPark name="down-square" :size="12" :gap="0" /></button>
    </div>
    <div v-if="open" class="cat-select-popup">
      <div class="cat-option" :class="{active: modelValue === ''}" @mousedown.prevent="pick('')">
        {{ t('未分类') }}
      </div>
      <div
        v-for="opt in options"
        :key="opt"
        class="cat-option"
        :class="{active: opt === modelValue}"
        @mousedown.prevent="pick(opt)"
      >{{ opt }}</div>
      <div v-if="options.length === 0" class="cat-option cat-empty">{{ t('暂无分类, 直接输入即可新建') }}</div>
    </div>
  </div>
</template>

<style scoped>
.cat-select {
  position: relative;
  width: 100%;
}
.cat-select-wrap {
  display: flex;
  align-items: stretch;
}
.cat-select-input {
  flex: 1 1 auto;
  border-top-right-radius: 0;
  border-bottom-right-radius: 0;
}
.cat-select-toggle {
  flex: 0 0 auto;
  width: 34px;
  border: 1px solid var(--border, #2a3344);
  border-left: none;
  border-top-right-radius: 6px;
  border-bottom-right-radius: 6px;
  background: var(--panel-3, #1b2230);
  color: var(--text-2, #aab4c4);
  cursor: pointer;
  font-size: 13px;
}
.cat-select-toggle:hover {
  background: var(--hover, #243042);
}
/* 弹层高度约为原生 datalist(~160px) 的 2 倍, 超出滚动。 */
.cat-select-popup {
  position: absolute;
  z-index: 50;
  left: 0;
  right: 0;
  top: calc(100% + 4px);
  max-height: 320px;
  overflow-y: auto;
  background: var(--panel-2, #161c28);
  border: 1px solid var(--border, #2a3344);
  border-radius: 6px;
  box-shadow: 0 6px 18px rgba(0, 0, 0, 0.4);
  padding: 4px;
}
.cat-option {
  padding: 6px 10px;
  border-radius: 4px;
  cursor: pointer;
  color: var(--text, #d0d8e4);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.cat-option:hover {
  background: var(--hover, #243042);
}
.cat-option.active {
  background: var(--accent, #3b82f6);
  color: #fff;
}
.cat-empty {
  color: var(--text-2, #aab4c4);
  cursor: default;
  font-style: italic;
}
.cat-empty:hover {
  background: transparent;
}
</style>
