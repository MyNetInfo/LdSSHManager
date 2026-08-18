<script lang="ts" setup>
import {reactive, onMounted, onUnmounted, computed, ref} from 'vue'
import SessionTree from './components/SessionTree.vue'
import TerminalTabs from './components/TerminalTabs.vue'
import FileManager from './components/FileManager.vue'
import Topbar from './components/Topbar.vue'
import LockScreen from './components/LockScreen.vue'
import VaultModal from './components/VaultModal.vue'
import QuickCommandBar from './components/QuickCommandBar.vue'
import QuickCommandPanel from './components/QuickCommandPanel.vue'
import Dialog from './components/Dialog.vue'
import {DuplicateSSH, DisconnectSSH, GetSetting, SetSetting} from '../wailsjs/go/main/App'
import {showToast} from './dialog'
import {t} from './i18n'
import {EventsOn} from '../wailsjs/runtime/runtime'
import {doLock, vaultState} from './services/vault'
import type {ActiveSession} from './types'

const state = reactive({
  sessions: [] as ActiveSession[],
  currentId: '',
})

const layout = reactive({
  leftWidth: 260,
  rightWidth: 260,
})

// 左侧面板底部标签: 'session' = 会话树, 'quick' = 快捷命令面板
const leftTab = ref('session')

const resizing = reactive({
  left: false,
  right: false,
})

// vault:unlocked 事件监听器句柄, 用于解锁后重读面板宽度(卸载时清理)。
let offUnlocked: (() => void) | null = null

// 自动锁屏: 监听用户活动, 距最后活动 N 分钟自动调 doLock
const autoLockMinutes = ref(0)
let lastActivity = Date.now()
let windowPaused = false  // 失焦时暂停计时
let checkTimer: number | null = null
const activityEvents = ['mousedown', 'keydown', 'touchstart', 'mousemove'] as const

async function loadAutoLock() {
  try {
    const v = await GetSetting('autoLockMinutes')
    autoLockMinutes.value = v ? Math.max(0, parseInt(v, 10) || 0) : 0
  } catch {
    autoLockMinutes.value = 0
  }
}

function bumpActivity() {
  lastActivity = Date.now()
}

function startAutoLockWatcher() {
  activityEvents.forEach(ev =>
    document.addEventListener(ev, bumpActivity, {passive: true}),
  )
  document.addEventListener('visibilitychange', () => {
    if (document.hidden) {
      windowPaused = true
    } else {
      windowPaused = false
      lastActivity = Date.now()  // 回来时重置, 不算闲置
    }
  })
  checkTimer = window.setInterval(() => {
    if (windowPaused) return
    if (autoLockMinutes.value <= 0) return
    if (!vaultState.enabled || vaultState.locked) return
    if (Date.now() - lastActivity >= autoLockMinutes.value * 60_000) {
      doLock()
    }
  }, 30_000)
}

// 左则快捷命令点击"添加"时, 若主面板偏窄则自动展开到 600px, 保证编辑区舒适。
// 展开后持久化, 下次打开仍是该宽度。
function expandLeftPanel() {
  if (layout.leftWidth < 500) {
    layout.leftWidth = 800
    saveWidths()
  }
}

// 左则快捷命令右键菜单点"编辑"时, 若主面板 < 500 则展开到 800px 给编辑区更大空间。
function expandLeftPanelEdit() {
  if (layout.leftWidth < 500) {
    layout.leftWidth = 800
    saveWidths()
  }
}

onUnmounted(() => {
  offUnlocked?.()
  window.removeEventListener('ldsshmanager:expand-left-panel', expandLeftPanel)
  window.removeEventListener('ldsshmanager:expand-left-panel-edit', expandLeftPanelEdit)
})

// 当前可接收快捷命令的会话 id: 仅当 currentId 对应会话处于 active 才返回,
// 否则返回 '' (快捷命令栏据此提示"请先连接一个会话")。
const activeSendId = computed(() => {
  const s = state.sessions.find((x) => x.id === state.currentId)
  return s && s.status === 'active' ? s.id : ''
})

onMounted(() => {
  loadWidths()
  // 启动若处于锁屏状态, 数据库未打开, 上面的 loadWidths 会失败(被 catch 吞掉);
  // 解锁后后端 emit vault:unlocked(数据库已打开), 这里重试一次恢复上次宽度。
  offUnlocked = EventsOn('vault:unlocked', () => loadWidths())
  // 左则快捷命令"添加"时, 主面板偏窄则自动展开到 600px。
  window.addEventListener('ldsshmanager:expand-left-panel', expandLeftPanel)
  // 左则快捷命令右键菜单点"编辑"时, 主面板偏窄则自动展开到 800px。
  window.addEventListener('ldsshmanager:expand-left-panel-edit', expandLeftPanelEdit)
  // 启动时若清理了上次异常退出的残留 WebView2 进程, 后端会推送 webview:cleaned 事件,
  // 这里弹一个提示告知用户(应用已正常启动)。
  EventsOn('webview:cleaned', (n: number) => {
    showToast(
      t('已自动清理上次残留的 WebView2 进程（{n} 个）', {n}),
      t('提示'),
      5000,
      'info',
    )
  })
  // 自动锁屏: 启动加载设置 + 监听设置变化 + 启动活动/定时器
  loadAutoLock()
  startAutoLockWatcher()
  window.addEventListener('ldsshmanager:settings-changed', (e: any) => {
    if (e?.detail?.key === 'autoLockMinutes') loadAutoLock()
  })
})

async function loadWidths() {
  try {
    const left = await GetSetting('ui.leftWidth')
    const right = await GetSetting('ui.rightWidth')
    if (left) layout.leftWidth = clamp(parseInt(left, 10), 160, 800)
    if (right) layout.rightWidth = clamp(parseInt(right, 10), 260, 800)
  } catch (err: any) {
    console.error('load widths:', err)
  }
}

async function saveWidths() {
  try {
    await SetSetting('ui.leftWidth', String(layout.leftWidth))
    await SetSetting('ui.rightWidth', String(layout.rightWidth))
  } catch (err: any) {
    console.error('save widths:', err)
  }
}

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value))
}

function onOpen(session: ActiveSession) {
  // 兼容旧调用(saveAndConnect 等不带 status) → 默认 active
  const item: ActiveSession = {...session, status: session.status || 'active'}
  if (!state.sessions.find((s) => s.id === item.id)) {
    state.sessions.push(item)
  }
  state.currentId = item.id
  saveLastSessions()
}

// 连接中标签 → 连接成功: 用真实 session 替换临时 id(标签立即变活跃, 终端开始渲染)
function onConnected(payload: {tempId: string; session: ActiveSession}) {
  const idx = state.sessions.findIndex((s) => s.id === payload.tempId)
  if (idx < 0) {
    // 标签已被用户关闭 → 断开刚建立的后端连接, 避免会话泄漏
    DisconnectSSH(payload.session.id).catch(() => {})
    return
  }
  state.sessions[idx] = {...payload.session}
  if (state.currentId === payload.tempId) {
    state.currentId = payload.session.id
  }
  saveLastSessions()
}

// 连接失败: 标签标记 failed(保留, 可查看错误/关闭)
function onFailed(payload: {tempId: string; error: string}) {
  const idx = state.sessions.findIndex((s) => s.id === payload.tempId)
  if (idx >= 0) {
    state.sessions[idx] = {...state.sessions[idx], status: 'failed', error: payload.error}
  }
}

function onSelect(id: string) {
  state.currentId = id
}

function onClose(id: string) {
  state.sessions = state.sessions.filter((s) => s.id !== id)
  if (state.currentId === id) {
    state.currentId = state.sessions.length ? state.sessions[state.sessions.length - 1].id : ''
  }
  saveLastSessions()
}

function onReorder({fromId, toId}: {fromId: string; toId: string}) {
  const fromIdx = state.sessions.findIndex((s) => s.id === fromId)
  const toIdx = state.sessions.findIndex((s) => s.id === toId)
  if (fromIdx < 0 || toIdx < 0 || fromIdx === toIdx) return
  const [moved] = state.sessions.splice(fromIdx, 1)
  state.sessions.splice(toIdx, 0, moved)
  saveLastSessions()
}

// saveLastSessions persists the ordered list of saved session ids so the app
// can restore the same tabs on next startup. Password values are never stored.
async function saveLastSessions() {
  try {
    const ids = state.sessions.map((s) => s.sessionId).filter((v) => v && v > 0)
    await SetSetting('ui.lastSessions', JSON.stringify(ids))
  } catch (err: any) {
    console.error('save last sessions:', err)
  }
}

async function onDuplicate(id: string) {
  try {
    const result = await DuplicateSSH(id)
    onOpen({...result, status: 'active'})
  } catch (err: any) {
    showToast(String(err), 'Duplicate failed', 5000, 'error')
  }
}

function startResizeLeft(e: MouseEvent) {
  e.preventDefault()
  resizing.left = true
  document.body.style.cursor = 'col-resize'
  window.addEventListener('mousemove', onMouseMove)
  window.addEventListener('mouseup', stopResize)
}

function startResizeRight(e: MouseEvent) {
  e.preventDefault()
  resizing.right = true
  document.body.style.cursor = 'col-resize'
  window.addEventListener('mousemove', onMouseMove)
  window.addEventListener('mouseup', stopResize)
}

function onMouseMove(e: MouseEvent) {
  if (!resizing.left && !resizing.right) return
  const layoutEl = document.getElementById('app-layout')
  if (!layoutEl) return
  const rect = layoutEl.getBoundingClientRect()
  if (resizing.left) {
    layout.leftWidth = clamp(e.clientX - rect.left, 160, Math.floor(rect.width * 0.8))
  }
  if (resizing.right) {
    layout.rightWidth = clamp(rect.right - e.clientX, 260, Math.floor(rect.width * 0.8))
  }
}

function stopResize() {
  resizing.left = false
  resizing.right = false
  document.body.style.cursor = ''
  window.removeEventListener('mousemove', onMouseMove)
  window.removeEventListener('mouseup', stopResize)
  saveWidths()
}

// 导入/云端覆盖后由 Topbar 触发, 刷新会话树
function refreshSessions() {
  window.dispatchEvent(new CustomEvent('ldsshmanager:refresh-sessions'))
}
</script>

<template>
  <div class="app-shell">
    <Topbar @refresh-sessions="refreshSessions" />
    <main id="app-layout">
      <div class="panel-left" :style="{width: layout.leftWidth + 'px'}">
        <div class="left-tab-bar">
          <button class="left-tab" :class="{active: leftTab === 'session'}" @click="leftTab = 'session'">{{ t('会话') }}</button>
          <button class="left-tab" :class="{active: leftTab === 'quick'}" @click="leftTab = 'quick'">{{ t('快捷命令') }}</button>
        </div>
        <div class="left-tab-body">
          <SessionTree
            v-show="leftTab === 'session'"
            class="left-pane"
            :active-ids="state.sessions.map((s) => s.id)"
            @open="onOpen"
            @connected="onConnected"
            @failed="onFailed"
          />
          <QuickCommandPanel v-show="leftTab === 'quick'" class="left-pane" :current-session-id="state.currentId" />
        </div>
      </div>
      <div class="divider" @mousedown="startResizeLeft"></div>
      <TerminalTabs
        class="panel-center"
        :sessions="state.sessions"
        :current-id="state.currentId"
        @select="onSelect"
        @close="onClose"
        @reorder="onReorder"
        @duplicate="onDuplicate"
      />
      <div class="divider" @mousedown="startResizeRight"></div>
      <FileManager
        class="panel-right"
        :style="{width: layout.rightWidth + 'px'}"
        :session-id="state.currentId"
      />
    </main>
    <QuickCommandBar :current-session-id="activeSendId" />
    <Dialog />
    <LockScreen />
    <VaultModal />
  </div>
</template>

<style>
.app-shell {
  display: flex;
  flex-direction: column;
  height: 100vh;
  width: 100%;
  overflow: hidden;
  background: var(--bg);
  color: var(--text);
}
#app-layout {
  display: flex;
  flex: 1;
  min-height: 0;
  width: 100%;
  overflow: hidden;
  background: var(--bg);
  color: var(--text);
}
.panel-left {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
  background: var(--panel);
}
.left-tab-bar {
  display: flex;
  flex-shrink: 0;
  height: 34px;
  border-bottom: 1px solid var(--border);
  background: var(--bg-2);
}
.left-tab {
  flex: 1;
  border: none;
  background: transparent;
  color: var(--text-2);
  cursor: pointer;
  border-bottom: 2px solid transparent;
}
.left-tab:hover {
  color: var(--text);
}
.left-tab.active {
  color: var(--primary);
  border-bottom-color: var(--primary);
  font-weight: 600;
}
.left-tab-body {
  flex: 1;
  min-height: 0;
  display: flex;
}
.left-pane {
  flex: 1;
  min-width: 0;
  min-height: 0;
}
.panel-center {
  flex: 1;
  min-width: 0;
}
.panel-right {
  flex-shrink: 0;
}
.divider {
  width: 5px;
  flex-shrink: 0;
  cursor: col-resize;
  background: transparent;
  transition: background 0.15s;
}
.divider:hover {
  background: var(--primary);
}
</style>
