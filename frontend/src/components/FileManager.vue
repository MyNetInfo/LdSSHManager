<script lang="ts" setup>
import {reactive, watch, ref, onMounted, onBeforeUnmount} from 'vue'
import {
  SFTPList,
  SFTPUpload,
  SFTPDownload,
  SFTPRename,
  SFTPDelete,
  SFTPMkdir,
  PickFile,
  PickSavePath,
} from '../../wailsjs/go/main/App'
import type {SFTPEntry} from '../types'
import {showPrompt, showConfirm, showToast} from '../dialog'
import {t} from '../i18n'
import IconPark from './IconPark.vue'

const props = defineProps<{
  sessionId: string
}>()

const state = reactive({
  path: '/root',
  entries: [] as SFTPEntry[],
  loading: false,
  error: '',
})

// 首次连接或断线重连(sessionId 变化)时的首次加载: 失败重试 3 次(连接刚建立易偶发失败)
const isFirstLoad = ref(false)

const contextMenu = reactive({
  show: false,
  x: 0,
  y: 0,
  entry: null as SFTPEntry | null,
})

watch(
  () => props.sessionId,
  () => {
    state.path = '/root'
    isFirstLoad.value = true
    load()
  },
  {immediate: true}
)

// 终端执行 cd 命令 → 联动切换本面板目录(TerminalPanel 广播 ldsshmanager:sftp-cd)
function onSftpCd(e: Event) {
  const d = (e as CustomEvent).detail
  if (!d || d.sessionId !== props.sessionId || !d.path) return
  if (d.path === state.path) return
  state.path = d.path
  load()
}
onMounted(() => {
  window.addEventListener('ldsshmanager:sftp-cd', onSftpCd)
})
onBeforeUnmount(() => {
  window.removeEventListener('ldsshmanager:sftp-cd', onSftpCd)
})

async function load() {
  if (!props.sessionId) {
    state.entries = []
    return
  }
  state.loading = true
  state.error = ''
  // 首次连接/断线重连(isFirstLoad=true)时: 失败重试 3 次(含第 1 次, 退避 400/800ms);
  // 其他正常 cd 切换路径失败仍只 1 次, 不掩盖用户操作的真实错误。
  const maxAttempts = isFirstLoad.value ? 3 : 1
  let lastErr: any = ''
  for (let attempt = 1; attempt <= maxAttempts; attempt++) {
    try {
      const result = await SFTPList(props.sessionId, state.path)
      state.path = result.path
      state.entries = result.entries
      state.loading = false
      isFirstLoad.value = false
      return
    } catch (err: any) {
      lastErr = err
      if (attempt < maxAttempts) {
        await new Promise<void>(r => setTimeout(r, 400 * attempt))
      }
    }
  }
  state.loading = false
  isFirstLoad.value = false
  state.error = String(lastErr)
}

function enter(entry: SFTPEntry) {
  if (!entry.isDir) return
  state.path = entry.path
  load()
}

function up() {
  if (state.path === '/') return
  const idx = state.path.lastIndexOf('/')
  if (idx <= 0) {
    state.path = '/'
  } else {
    state.path = state.path.slice(0, idx)
  }
  load()
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
}

function formatTime(ms: number): string {
  return new Date(ms).toLocaleString()
}

async function upload() {
  if (!props.sessionId) return
  const local = await PickFile()
  if (!local) return
  const name = local.split(/[\\/]/).pop() || 'file'
  const remote = (state.path === '/' ? '/' : state.path + '/') + name
  try {
    await SFTPUpload(props.sessionId, local, remote)
    await load()
  } catch (err: any) {
    state.error = String(err)
  }
}

async function newFolder() {
  if (!props.sessionId) return
  const name = await showPrompt(t('文件夹名称'), '', t('新建文件夹'))
  if (!name) return
  const remote = (state.path === '/' ? '/' : state.path + '/') + name
  try {
    await SFTPMkdir(props.sessionId, remote)
    await load()
  } catch (err: any) {
    state.error = String(err)
  }
}

async function download(entry: SFTPEntry) {
  if (!props.sessionId || entry.isDir) return
  const local = await PickSavePath(entry.name)
  if (!local) return
  try {
    await SFTPDownload(props.sessionId, entry.path, local)
  } catch (err: any) {
    state.error = String(err)
  }
}

async function renameEntry(entry: SFTPEntry) {
  if (!props.sessionId) return
  const base = entry.path.split(/[\\/]/).pop() || ''
  const next = await showPrompt(t('重命名为'), base, t('重命名为'))
  if (!next || next === base) return
  const parent = entry.path.slice(0, entry.path.length - base.length)
  const newPath = parent + next
  try {
    await SFTPRename(props.sessionId, entry.path, newPath)
    await load()
  } catch (err: any) {
    state.error = String(err)
  }
}

async function removeEntry(entry: SFTPEntry) {
  if (!props.sessionId) return
  const kind = entry.isDir ? t('删除文件夹 "{name}"？', {name: entry.name}) : t('删除文件 "{name}"？', {name: entry.name})
  const ok = await showConfirm(kind, t('确认'), t('删除'), t('取消'))
  if (!ok) return
  try {
    await SFTPDelete(props.sessionId, entry.path)
    await load()
  } catch (err: any) {
    state.error = String(err)
  }
}

function openContextMenu(e: MouseEvent, entry: SFTPEntry) {
  e.preventDefault()
  contextMenu.x = e.clientX
  contextMenu.y = e.clientY
  contextMenu.entry = entry
  contextMenu.show = true
}

function closeContextMenu() {
  contextMenu.show = false
  contextMenu.entry = null
}

async function copyFullPath() {
  const path = contextMenu.entry?.path
  closeContextMenu()
  if (!path) return
  try {
    await navigator.clipboard.writeText(path)
    showToast(t('已复制完整路径'), '', 5000, 'info')
  } catch {
    showToast(t('复制失败'), '', 5000, 'error')
  }
}
</script>

<template>
  <aside class="file-manager">
    <header class="fm-header">
      <span>{{ t('文件管理器') }}</span>
    </header>
    <div class="fm-toolbar">
      <button class="fm-btn fm-btn-text" @click="up" :disabled="state.path === '/'" :title="t('上级目录')"><IconPark name="up" :size="13" :gap="2" />{{ t('上级目录') }}</button>
      <span class="fm-path">{{ state.path }}</span>
      <button class="fm-btn fm-btn-text" @click="newFolder" :title="t('新建文件夹')"><IconPark name="add" :size="13" :gap="2" />{{ t('新建文件夹') }}</button>
      <button class="fm-btn fm-btn-text" @click="upload" :title="t('上传')"><IconPark name="upload" :size="13" :gap="2" />{{ t('上传') }}</button>
      <button class="fm-btn fm-btn-text" @click="load" :title="t('刷新')"><IconPark name="refresh" :size="13" :gap="2" />{{ t('刷新') }}</button>
    </div>
    <div v-if="!props.sessionId" class="fm-empty">{{ t('无活动会话') }}</div>
    <div v-else-if="state.loading" class="fm-empty">{{ t('加载中...') }}</div>
    <div v-else-if="state.error" class="fm-empty fm-error">{{ state.error }}</div>
    <div v-else class="fm-table-wrap">
      <table class="fm-table">
      <thead>
        <tr>
          <th class="col-name">{{ t('名称') }}</th>
          <th class="col-size">{{ t('大小') }}</th>
          <th class="col-time">{{ t('修改时间') }}</th>
          <th class="col-actions">{{ t('操作') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="e in state.entries" :key="e.path" @dblclick="enter(e)" @contextmenu.prevent="openContextMenu($event, e)">
          <td class="col-name">
            <span class="file-icon" :class="{folder: e.isDir}"><IconPark :name="e.isDir ? 'folder' : 'file'" :size="15" :gap="4" /></span>
            <span :title="e.name">{{ e.name }}</span>
          </td>
          <td class="col-size">{{ e.isDir ? '-' : formatSize(e.size) }}</td>
          <td class="col-time">{{ formatTime(e.modTime) }}</td>
          <td class="col-actions">
            <button v-if="!e.isDir" class="op-btn op-btn-text" :title="t('下载')" @click.stop="download(e)"><IconPark name="download" :size="13" :gap="2" />{{ t('下载') }}</button>
            <button class="op-btn op-btn-text" :title="t('重命名为')" @click.stop="renameEntry(e)"><IconPark name="edit" :size="13" :gap="2" />{{ t('重命名') }}</button>
            <button class="op-btn op-btn-text del" :title="t('删除')" @click.stop="removeEntry(e)"><IconPark name="delete" :size="13" :gap="2" />{{ t('删除') }}</button>
          </td>
        </tr>
      </tbody>
      </table>
    </div>

    <div v-if="contextMenu.show" class="context-overlay" @click="closeContextMenu"></div>
    <div
      v-if="contextMenu.show"
      class="context-menu"
      :style="{left: contextMenu.x + 'px', top: contextMenu.y + 'px'}"
      @click.stop
    >
      <div class="context-item" @click="copyFullPath">{{ t('复制完整路径') }}</div>
    </div>
  </aside>
</template>

<style scoped>
.file-manager {
  width: 320px;
  min-width: 260px;
  background: var(--panel);
  color: var(--text);
  border-left: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.fm-header {
  padding: 8px 12px;
  font-weight: 700;
  background: var(--panel-2);
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.fm-toolbar {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.fm-btn {
  width: 24px;
  height: 22px;
  border: 1px solid var(--border-2);
  background: var(--hover-2);
  color: var(--text);
  cursor: pointer;
  border-radius: 3px;
}
.fm-btn:disabled {
  opacity: 0.4;
  cursor: default;
}
/* 图标+文字变体: 自适应宽度, 统一功能按钮风格(图标+文字) */
.fm-btn-text,
.op-btn-text {
  width: auto;
  padding: 0 6px;
  display: inline-flex;
  align-items: center;
  font-size: 12px;
  white-space: nowrap;
}
.fm-path {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-2);
}
.fm-empty {
  padding: 20px;
  text-align: center;
  color: var(--text-3);
}
.fm-error {
  color: var(--danger);
}
.fm-table-wrap {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  min-height: 0;
}
.fm-table {
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
}
.fm-table th {
  position: sticky;
  top: 0;
  background: var(--panel-2);
  text-align: left;
  padding: 6px 8px;
  border-bottom: 1px solid var(--border);
  color: var(--text-2);
  font-weight: 600;
}
.fm-table td {
  padding: 5px 8px;
  border-bottom: 1px solid var(--border);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.fm-table tr:hover td {
  background: var(--hover);
}
.col-name {
  width: 55%;
}
.col-size {
  width: 20%;
  text-align: right;
}
.col-time {
  width: 25%;
}
.col-actions {
  width: 38%;
  text-align: center;
}
.op-btn {
  border: 1px solid var(--border-2);
  background: var(--hover-2);
  color: var(--text);
  border-radius: 3px;
  cursor: pointer;
  width: 24px;
  height: 22px;
  margin: 0 2px;
  line-height: 1;
}
.op-btn:hover {
  background: var(--hover-2);
}
.op-btn.del:hover {
  background: var(--danger-hover);
  border-color: var(--danger);
  color: var(--danger);
}
.file-icon {
  margin-right: 6px;
}
.file-icon.folder {
  color: var(--warn);
}
.context-overlay {
  position: fixed;
  inset: 0;
  z-index: 100;
}
.context-menu {
  position: fixed;
  z-index: 101;
  min-width: 140px;
  background: var(--panel-2);
  border: 1px solid var(--border-2);
  border-radius: 4px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.35);
  padding: 4px 0;
  color: var(--text);
}
.context-item {
  padding: 7px 14px;
  cursor: pointer;
  white-space: nowrap;
}
.context-item:hover {
  background: var(--hover);
}
</style>
