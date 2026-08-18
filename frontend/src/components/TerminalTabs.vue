<script lang="ts" setup>
import {computed} from 'vue'
import TerminalPanel from './TerminalPanel.vue'
import SessionStatsBar from './SessionStatsBar.vue'
import {DisconnectSSH} from '../../wailsjs/go/main/App'
import type {ActiveSession} from '../types'
import {t} from '../i18n'
import IconPark from './IconPark.vue'

const props = defineProps<{
  sessions: ActiveSession[]
  currentId: string
}>()

const emit = defineEmits<{
  (e: 'select', id: string): void
  (e: 'close', id: string): void
  (e: 'reorder', payload: {fromId: string; toId: string}): void
  (e: 'duplicate', id: string): void
}>()

const displaySessions = computed(() => {
  const counts = new Map<string, number>()
  return props.sessions.map((s) => {
    const c = counts.get(s.name) || 0
    counts.set(s.name, c + 1)
    return {...s, label: c === 0 ? s.name : `${s.name} (${c})`}
  })
})

function selectTab(id: string) {
  emit('select', id)
}

async function closeTab(id: string) {
  await DisconnectSSH(id)
  emit('close', id)
}

function onDragStart(e: DragEvent, id: string) {
  if (!e.dataTransfer) return
  e.dataTransfer.setData('text/plain', id)
  e.dataTransfer.effectAllowed = 'move'
}

function onDrop(e: DragEvent, targetId: string) {
  e.preventDefault()
  const fromId = e.dataTransfer?.getData('text/plain')
  if (!fromId || fromId === targetId) return
  emit('reorder', {fromId, toId: targetId})
}

function duplicateTab(id: string) {
  emit('duplicate', id)
}
</script>

<template>
  <section class="terminal-tabs">
    <div class="tab-bar">
      <div
        v-for="s in displaySessions"
        :key="s.id"
        class="tab"
        :class="{active: s.id === props.currentId}"
        draggable="true"
        @click="selectTab(s.id)"
        @dblclick.prevent="duplicateTab(s.id)"
        @dragstart="onDragStart($event, s.id)"
        @dragover.prevent
        @drop="onDrop($event, s.id)"
      >
        <span class="tab-name">{{ s.label }}</span>
        <button class="tab-close" @click.stop="closeTab(s.id)" @dblclick.stop><IconPark name="close" :size="11" :gap="0" /></button>
      </div>
    </div>
    <div class="tab-content">
      <SessionStatsBar :session-id="props.currentId" />
      <div class="tab-content-main">
        <template v-for="s in props.sessions" :key="s.id">
          <TerminalPanel
            v-show="s.id === props.currentId"
            :session-id="s.id"
            :focused="s.id === props.currentId"
            :status="s.status"
            :error="s.error"
            :host="s.host"
          />
        </template>
        <div v-if="props.sessions.length === 0" class="empty-term">
          <p>{{ t('双击左侧会话进行连接') }}</p>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.terminal-tabs {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  background: var(--bg);
}
.tab-bar {
  display: flex;
  align-items: center;
  height: 32px;
  background: var(--panel);
  border-bottom: 1px solid var(--border);
  overflow-x: auto;
}
.tab {
  display: flex;
  align-items: center;
  min-width: 120px;
  max-width: 200px;
  height: 31px;
  padding: 0 10px;
  background: var(--scroll-track);
  border-right: 1px solid var(--border);
  cursor: default;
  color: var(--text-2);
  transition: background 0.15s, color 0.15s;
  box-sizing: border-box;
}
.tab:hover {
  cursor: pointer;
  background: var(--panel-2);
  color: var(--text);
}
.tab[draggable="true"]:active {
  cursor: grabbing;
}
.tab.active {
  position: relative;
  z-index: 1;
  height: 32px;
  background: var(--panel-3);
  color: var(--text);
  font-weight: 600;
  border-bottom: 3px solid var(--primary);
}
.tab-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.tab-close {
  background: transparent;
  border: none;
  color: var(--text-2);
  cursor: pointer;
  margin-left: 4px;
  padding: 6px 8px;
  min-width: 24px;
  min-height: 24px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  font-size: 14px;
  line-height: 1;
  transition: background-color 0.12s ease;
}
.tab-close:hover {
  background: var(--hover);
  color: var(--danger);
}
.tab.active .tab-close {
  color: var(--text);
}
.tab-close:hover {
  color: var(--danger);
}
.tab-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  position: relative;
  min-height: 0;
}
.tab-content-main {
  flex: 1;
  position: relative;
  min-height: 0;
}
.empty-term {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-3);
}
</style>
