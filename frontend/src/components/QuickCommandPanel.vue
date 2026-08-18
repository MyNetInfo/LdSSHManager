<script lang="ts" setup>
import {onMounted, onBeforeUnmount, ref, reactive, computed, nextTick} from 'vue'
import {
  ListQuickCommands,
  SaveQuickCommand,
  DeleteQuickCommand,
  SSHSendData,
} from '../../wailsjs/go/main/App'
import {showToast, showConfirm} from '../dialog'
import {t} from '../i18n'
import type {QuickCommand} from '../types'
import CategorySelect from './CategorySelect.vue'
import IconPark from './IconPark.vue'

const props = defineProps<{
  currentSessionId?: string
}>()

const commands = ref<QuickCommand[]>([])

// 当前选中的快捷命令 id(单击选中+右侧编辑; 双击执行)。
const selectedId = ref<number | null>(null)

// 右侧编辑区当前编辑的命令 id: null=无/占位, 0=新增模式, >0=编辑已有。
const editingId = ref<number | null>(null)

// 右侧编辑区表单数据。
const editing = reactive({
  name: '',
  command: '',
  withEnter: true,
  category: '',
})

// 名称输入框引用(用于聚焦)。
const nameInputRef = ref<HTMLInputElement | null>(null)

// 当前编辑项的"初始快照"(JSON 字符串): 用于检测是否有未保存修改。
// select/openAdd 时记录初始值, saveEdit/删除/清空后重置; dirty=true 时保存按钮变红。
let originalSnapshot = ''

function snapshotOf(rec: {name: string; command: string; withEnter: boolean; category: string}): string {
  return JSON.stringify({name: rec.name, command: rec.command, withEnter: rec.withEnter, category: rec.category})
}

// 编辑区内容与初始快照不一致 = 有未保存的修改。
const dirty = computed(
  () =>
    snapshotOf({name: editing.name, command: editing.command, withEnter: editing.withEnter, category: editing.category}) !==
    originalSnapshot,
)

// 把命令数据填入右侧编辑区。可选触发指定的主面板宽度展开事件。
function applySelect(cmd: QuickCommand, expandEvent?: string) {
  selectedId.value = cmd.id
  editingId.value = cmd.id
  editing.name = cmd.name
  editing.command = cmd.command
  editing.withEnter = cmd.withEnter
  editing.category = cmd.category || ''
  originalSnapshot = snapshotOf(editing)
  if (expandEvent) window.dispatchEvent(new Event(expandEvent))
}

// 单击列表项: 仅填入编辑区, 不改变主面板宽度。
function select(cmd: QuickCommand) {
  applySelect(cmd)
}

// 右键菜单"编辑": 填入编辑区, 若主面板 < 500 则自动展开到 800px(给编辑区更宽空间)。
function editCommand(cmd: QuickCommand) {
  applySelect(cmd, 'ldsshmanager:expand-left-panel-edit')
}

async function send(cmd: QuickCommand) {
  selectedId.value = cmd.id
  if (!props.currentSessionId) {
    showToast(t('请先连接一个会话'), '', 5000, 'error')
    return
  }
  const text = cmd.withEnter ? cmd.command + '\r' : cmd.command
  try {
    await SSHSendData(props.currentSessionId, text)
  } catch (err: any) {
    showToast(String(err), '', 5000, 'error')
  }
}

// 右键上下文菜单: 删除/复制 两项(编辑已由右侧内联完成)。
const ctxMenu = reactive({
  visible: false,
  x: 0,
  y: 0,
  cmd: null as QuickCommand | null,
})

function onCtxMenu(e: MouseEvent, cmd: QuickCommand) {
  ctxMenu.cmd = cmd
  ctxMenu.x = e.clientX
  ctxMenu.y = e.clientY
  ctxMenu.visible = true
}

function closeCtxMenu() {
  ctxMenu.visible = false
  ctxMenu.cmd = null
}

// 分类筛选器; 空串=全部分类。
const filterCategory = ref('')

async function load() {
  try {
    commands.value = (await ListQuickCommands()) || []
    // 重载后若当前编辑的 id 已不在列表中, 清空编辑区。
    if (editingId.value !== null && editingId.value !== 0) {
      const stillExists = commands.value.some(c => c.id === editingId.value)
      if (!stillExists) {
        editingId.value = null
        editing.name = ''
        editing.command = ''
        editing.withEnter = true
        editing.category = ''
        originalSnapshot = ''
      }
    }
  } catch (err: any) {
    console.error('load quick commands:', err)
  }
}

// 任一处(左则/底部)增删改快捷命令后, 派发事件让另一处同步刷新。
function notifyChanged() {
  window.dispatchEvent(new Event('ldsshmanager:refresh-quick-commands'))
}

onMounted(() => {
  load()
  window.addEventListener('ldsshmanager:refresh-sessions', load)
  window.addEventListener('ldsshmanager:refresh-quick-commands', load)
  window.addEventListener('click', closeCtxMenu)
  window.addEventListener('blur', closeCtxMenu)
})

onBeforeUnmount(() => {
  window.removeEventListener('ldsshmanager:refresh-sessions', load)
  window.removeEventListener('ldsshmanager:refresh-quick-commands', load)
  window.removeEventListener('click', closeCtxMenu)
  window.removeEventListener('blur', closeCtxMenu)
})

// 已使用的分类(去重), 供筛选下拉与编辑区使用。
const categoryOptions = computed(() => {
  const set = new Set<string>()
  for (const c of commands.value) {
    const cat = (c.category || '').trim()
    if (cat) set.add(cat)
  }
  return Array.from(set).sort((a, b) => a.localeCompare(b))
})

// 按分类分组(空分类归为"未分类"), 分类按名称排序, 组内按命令名排序; 未分类排最后。
const grouped = computed(() => {
  const map = new Map<string, QuickCommand[]>()
  for (const c of commands.value) {
    const raw = (c.category || '').trim()
    const key = raw === '' ? '__none__' : raw
    if (!map.has(key)) map.set(key, [])
    map.get(key)!.push(c)
  }
  const out: {key: string; label: string; items: QuickCommand[]}[] = []
  for (const [key, items] of map.entries()) {
    out.push({key, label: key === '__none__' ? t('未分类') : key, items})
  }
  out.sort((a, b) => {
    if (a.key === '__none__') return 1
    if (b.key === '__none__') return -1
    return a.label.localeCompare(b.label)
  })
  for (const g of out) {
    g.items.sort((a, b) => a.name.localeCompare(b.name))
  }
  return out
})

// 点击"+ 添加": 清空编辑区进入新增模式, 聚焦名称输入框。
function openAdd() {
  editingId.value = 0
  selectedId.value = null
  editing.name = ''
  editing.command = ''
  editing.withEnter = true
  editing.category = ''
  originalSnapshot = snapshotOf(editing)
  nextTick(() => nameInputRef.value?.focus())
  // 编辑区需要足够宽度才能舒适显示, 若左则主面板偏窄则自动展开到 500px。
  window.dispatchEvent(new Event('ldsshmanager:expand-left-panel'))
}

async function saveEdit() {
  const name = editing.name.trim()
  if (!name) {
    showToast(t('请输入名称'), '', 5000, 'error')
    return
  }
  const payload: QuickCommand = {
    id: editingId.value || 0,
    name,
    command: editing.command,
    withEnter: editing.withEnter,
    sortOrder: 0,
    category: editing.category.trim(),
  }
  try {
    await SaveQuickCommand(payload)
    notifyChanged()
    // 保存成功后刷新列表, 保持当前项选中状态(新建的 id 会变, 由刷新后的列表匹配名称来恢复)。
    await load()
    // 尝试按名称找到刚保存的项并保持选中。
    const saved = commands.value.find(c => c.name === name)
    if (saved) {
      selectedId.value = saved.id
      editingId.value = saved.id
      editing.name = saved.name
      editing.command = saved.command
      editing.withEnter = saved.withEnter
      editing.category = saved.category || ''
    }
    // 归一化编辑区(名称/分类已 trim)并重置快照 → dirty 变 false, 保存按钮恢复原色。
    editing.name = editing.name.trim()
    editing.category = editing.category.trim()
    originalSnapshot = snapshotOf(editing)
    showToast(t('已保存'), '', 3000, 'success')
  } catch (err: any) {
    showToast(String(err), '', 5000, 'error')
  }
}

// 右键删除: 删除指定命令并清空编辑区(若正在编辑该命令)。
async function remove(cmd: QuickCommand) {
  const ok = await showConfirm(
    t('删除') + '「' + cmd.name + '」?',
    t('快捷命令'),
    t('删除'),
    t('取消'),
  )
  if (!ok) return
  try {
    await DeleteQuickCommand(cmd.id)
    notifyChanged()
    // 若当前编辑区正是被删的项, 清空编辑区。
    if (editingId.value === cmd.id) {
      editingId.value = null
      selectedId.value = null
      editing.name = ''
      editing.command = ''
      editing.withEnter = true
      editing.category = ''
      originalSnapshot = ''
    }
  } catch (err: any) {
    showToast(String(err), '', 5000, 'error')
  }
}

// 复制快捷命令的命令文本到剪贴板
async function copyCommand(cmd: QuickCommand) {
  const text = cmd.command || ''
  if (!text) {
    showToast(t('命令为空, 无可复制内容'), '', 5000, 'info')
    return
  }
  try {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      await navigator.clipboard.writeText(text)
    } else {
      const ta = document.createElement('textarea')
      ta.value = text
      ta.style.position = 'fixed'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
    }
    showToast(t('已复制') + ': ' + text, '', 5000, 'info')
  } catch (err: any) {
    showToast(String(err), '', 5000, 'error')
  }
}
</script>

<template>
  <section class="qc-panel">
    <!-- 左侧: 分类列表 -->
    <div class="qc-left">
      <div class="qc-toolbar">
        <select class="qc-filter" v-model="filterCategory" :title="t('分类')">
          <option value="">{{ t('全部分类') }}</option>
          <option v-for="c in categoryOptions" :key="c" :value="c">{{ c }}</option>
        </select>
        <button class="btn btn-primary qc-add" @click="openAdd"><IconPark name="add" :size="14" /> {{ t('添加') }}</button>
      </div>

      <div class="qc-tree">
        <details v-for="g in grouped" :key="g.key" class="qc-group" name="qc-categories">
          <summary class="qc-group-summary">
            <span class="qc-group-name">{{ g.label }}</span>
            <span class="qc-group-count">{{ g.items.length }}</span>
          </summary>
          <div
            v-for="(cmd, idx) in g.items.filter(c => !filterCategory || (c.category || '') === filterCategory)"
            :key="cmd.id"
            class="qc-row"
            :class="{ 'qc-row-selected': selectedId === cmd.id }"
            @contextmenu.prevent="onCtxMenu($event, cmd)"
            @click="select(cmd)"
            @dblclick="send(cmd)"
          >
            <div class="qc-row-head">
              <span class="qc-row-name">{{ cmd.name }}</span>
            </div>
          </div>
        </details>
        <div v-if="commands.length === 0" class="qc-empty">{{ t('没有快捷命令, 点击新增添加') }}</div>
      </div>
    </div>

    <!-- 右侧: 内联编辑区 -->
    <div class="qc-editor">
      <!-- 未选中时的占位提示 -->
      <div v-if="editingId === null" class="qc-editor-placeholder">
        <p>{{ t('选择一条快捷命令进行编辑') }}</p>
        <p class="qc-placeholder-sub">或点击「{{ t('添加') }}」创建新命令</p>
      </div>

      <!-- 编辑表单 -->
      <template v-else>
        <div class="qc-editor-header">
          <h3 class="qc-editor-title">{{ editingId === 0 ? t('添加快捷命令') : t('编辑快捷命令') }}</h3>
        </div>
        <div class="qc-editor-body">
          <div class="form-row">
            <label class="form-label">{{ t('名称') }}</label>
            <input
              ref="nameInputRef"
              v-model="editing.name"
              class="form-input"
              :placeholder="t('按钮上显示的文字')"
              @keyup.enter="saveEdit"
            />
          </div>
          <div class="form-row">
            <label class="form-label">{{ t('分类') }}</label>
            <CategorySelect
              v-model="editing.category"
              :options="categoryOptions"
              :placeholder="t('未分类')"
              @enter="saveEdit"
            />
          </div>
          <div class="form-row qc-cmd-row">
            <label class="form-label">{{ t('命令') }}</label>
            <textarea
              v-model="editing.command"
              class="form-input qc-cmd-input"
              :placeholder="t('点击按钮时发送到终端, 如: ls -la')"
              @keydown.ctrl.s.prevent="saveEdit"
              @keydown.meta.s.prevent="saveEdit"
            ></textarea>
          </div>
          <div class="form-row">
            <label class="form-label">{{ t('附加回车') }}</label>
            <label class="qc-switch">
              <input type="checkbox" v-model="editing.withEnter" />
              <span>{{ t('发送后自动执行') }}</span>
            </label>
          </div>
        </div>
        <div class="qc-editor-footer">
          <button
            v-if="editingId !== 0 && editingId !== null"
            class="btn btn-danger qc-btn-del"
            @click="remove(commands.find(c => c.id === editingId)!)"
          >{{ t('删除') }}</button>
          <div class="qc-editor-actions">
            <button
              class="btn btn-primary qc-btn-save"
              :class="{ 'qc-save-dirty': dirty }"
              @click="saveEdit"
            >{{ t('保存') }}</button>
          </div>
        </div>
      </template>
    </div>

    <!-- 右键上下文菜单: 复制 / 编辑 / 删除 -->
    <teleport to="body">
      <div
        v-if="ctxMenu.visible && ctxMenu.cmd"
        class="qc-ctx-menu"
        :style="{left: ctxMenu.x + 'px', top: ctxMenu.y + 'px'}"
        @click.stop
      >
        <div class="qc-ctx-item qc-ctx-copy" @click="copyCommand(ctxMenu.cmd!); closeCtxMenu()">{{ t('复制') }}</div>
        <div class="qc-ctx-item qc-ctx-edit" @click="editCommand(ctxMenu.cmd!); closeCtxMenu()">{{ t('编辑') }}</div>
        <div class="qc-ctx-item qc-ctx-del" @click="remove(ctxMenu.cmd!); closeCtxMenu()">{{ t('删除') }}</div>
      </div>
    </teleport>
  </section>
</template>

<style scoped>
.qc-panel {
  display: flex;
  flex-direction: row;
  height: 100%;
  min-height: 0;
  background: var(--panel);
}

/* ======== 左侧列表 ======== */
.qc-left {
  display: flex;
  flex-direction: column;
  width: 260px;
  min-width: 180px;
  max-width: 360px;
  border-right: 1px solid var(--border);
  flex-shrink: 0;
}
.qc-toolbar {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 8px;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.qc-filter {
  flex: 1;
  min-width: 0;
  height: 28px;
  background: var(--input-bg);
  color: var(--text);
  border: 1px solid var(--border-2);
  border-radius: 5px;
  padding: 0 6px;
}
.qc-add {
  flex-shrink: 0;
  height: 28px;
  padding: 0 10px;
  font-size: 13px;
}
.qc-tree {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 6px 4px;
}
.qc-group {
  margin-bottom: 4px;
}
.qc-group-summary {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  cursor: pointer;
  user-select: none;
  font-weight: 600;
  color: var(--text-2);
  background: var(--bg-2);
  border-radius: 5px;
}
.qc-group-count {
  font-weight: 400;
  color: var(--text-3);
}
.qc-row {
  padding: 5px 8px 6px 12px;
  border-left: 2px solid var(--border);
  margin: 2px 0 2px 4px;
  cursor: pointer;
}
.qc-row-selected {
  background: var(--hover);
  border-left-color: var(--accent);
}
.qc-row-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
}
.qc-row-name {
  font-weight: 500;
  color: var(--text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
}
.qc-empty {
  padding: 16px 8px;
  text-align: center;
  color: var(--text-3);
  font-size: 13px;
}

/* ======== 右侧编辑区 ======== */
.qc-editor {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}
.qc-editor-placeholder {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: var(--text-3);
  gap: 8px;
  user-select: none;
}
.qc-placeholder-sub {
  font-size: 12px;
  color: var(--text-3);
  opacity: 0.7;
}
.qc-editor-header {
  flex-shrink: 0;
  padding: 10px 16px 6px;
  border-bottom: 1px solid var(--border);
}
.qc-editor-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--text);
}
.qc-editor-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 12px 16px;
  overflow: hidden;
}
.qc-editor-footer {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 16px;
  border-top: 1px solid var(--border);
}
.qc-editor-actions {
  display: flex;
  gap: 8px;
  margin-left: auto;
}
.qc-btn-del {
  font-size: 13px;
}
.qc-btn-save {
  /* 相比普通按钮(padding 7px 16px)加宽 1.5 倍 */
  min-width: 112px;
  justify-content: center;
}
/* 有未保存修改时保存按钮变红, 保存成功后由 dirty=false 还原 */
.qc-save-dirty,
.qc-save-dirty:hover {
  background: #e5534b !important;
  border-color: #e5534b !important;
  color: #fff;
}

/* 表单行 */
.form-row {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 10px;
}
.form-label {
  width: 60px;
  flex-shrink: 0;
  text-align: right;
  color: var(--text-2);
  font-size: 13px;
}
.form-input {
  flex: 1;
  min-width: 0;
  padding: 7px 10px;
  background: var(--input-bg);
  color: var(--text);
  border: 1px solid var(--border-2);
  border-radius: 6px;
  outline: none;
  font-size: 13px;
}
.form-input:focus {
  border-color: var(--primary);
}
.qc-cmd-row {
  flex: 1;
  min-height: 0;
  align-items: stretch;
  margin-bottom: 8px;
}
.qc-cmd-row .form-label {
  align-self: flex-start;
  padding-top: 9px;
}
.qc-cmd-input {
  resize: none;
  font-family: Consolas, "Courier New", monospace;
  font-size: 13px;
  line-height: 1.5;
  flex: 1;
  min-height: 0;
}
.qc-switch {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--text-2);
  cursor: pointer;
  font-size: 13px;
}
.qc-switch input {
  width: 16px;
  height: 16px;
  cursor: pointer;
}

/* 右键菜单 */
.qc-ctx-menu {
  position: fixed;
  z-index: 1000;
  min-width: 120px;
  background: var(--panel);
  color: var(--text);
  border: 1px solid var(--border-2);
  border-radius: 6px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.25);
  padding: 4px 0;
  user-select: none;
}
.qc-ctx-item {
  padding: 7px 14px;
  cursor: pointer;
  white-space: nowrap;
  font-size: 13px;
}
.qc-ctx-item:hover {
  background: var(--hover);
}
.qc-ctx-del {
  color: var(--danger);
}
.qc-ctx-del:hover {
  background: var(--danger-hover);
  color: #fff;
}
</style>
