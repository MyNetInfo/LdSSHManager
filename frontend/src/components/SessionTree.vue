<script lang="ts" setup>
import {reactive, onMounted, onBeforeUnmount, ref} from 'vue'
import {EventsOn} from '../../wailsjs/runtime/runtime'
import {
  ListSavedSessions,
  SaveSession,
  DeleteSession,
  DuplicateSavedSession,
  ConnectSSH,
  TrustHostKey,
  ResetHostKey,
  PickFile,
  GetSetting,
  SetSetting,
} from '../../wailsjs/go/main/App'
import type {ActiveSession, SavedSession} from '../types'
import {showConfirm, showPrompt, showToast} from '../dialog'
import {t} from '../i18n'
import IconPark from './IconPark.vue'

const props = defineProps<{
  activeIds: string[]
}>()

const emit = defineEmits<{
  (e: 'open', session: ActiveSession): void
  (e: 'connected', payload: {tempId: string; session: ActiveSession}): void
  (e: 'failed', payload: {tempId: string; error: string}): void
}>()

interface Group {
  name: string
  sessions: SavedSession[]
  expanded: boolean
}

interface SessionForm {
  id: number
  name: string
  host: string
  port: number
  username: string
  authType: 'password' | 'key'
  keyPath: string
  groupName: string
  sortOrder: number
  password: string
  proxyType: 'none' | 'socks5' | 'http'
  proxyHost: string
  proxyPort: number
  proxyUsername: string
  proxyPassword: string
  execCmd: string
}

const state = reactive({
  groups: [] as Group[],
  showForm: false,
  formMode: 'add' as 'add' | 'edit',
  form: {
    id: 0,
    name: '',
    host: '',
    port: 22,
    username: 'root',
    authType: 'password',
    keyPath: '',
    groupName: 'default',
    sortOrder: 0,
    password: '',
    proxyType: 'none',
    proxyHost: '',
    proxyPort: 1080,
    proxyUsername: '',
    proxyPassword: '',
    execCmd: '',
  } as SessionForm,
})

const contextMenu = reactive({
  show: false,
  x: 0,
  y: 0,
  session: null as SavedSession | null,
})

// 当前选中的会话 id(单击选中, 高亮显示; 双击连接)。
const selectedId = ref<number | null>(null)

function selectSession(s: SavedSession) {
  selectedId.value = s.id
}

// restored 标记"上次打开的会话已尝试恢复完成"(无论有没有数据)。
// 有锁屏密码时启动处于锁定态(DB 未打开)恢复会失败 → restored=false,
// 解锁后由 vault:unlocked 事件重试一次; 无密码启动直接成功 → 不再重复。
let restored = false
// offUnlocked 解锁事件监听(解锁成功后恢复上次打开的会话)
let offUnlocked: (() => void) | null = null

onMounted(async () => {
  await loadSessions()
  await tryRestore()
  // 导入数据 / 云端覆盖本地 后由 App 广播刷新
  window.addEventListener('ldsshmanager:refresh-sessions', loadSessions)
  // 失败页"重置并重连": 复用原标签(id 不变)重新发起连接
  window.addEventListener('ldsshmanager:reconnect-session', onReconnectSession)
  // 解锁成功(设了锁屏密码时启动为锁定态): 数据库刚打开, 重新加载并恢复上次会话
  offUnlocked = EventsOn('vault:unlocked', async () => {
    await loadSessions()
    await tryRestore()
  })
})

// cleanup 在组件卸载时移除事件监听
onBeforeUnmount(() => {
  window.removeEventListener('ldsshmanager:refresh-sessions', loadSessions)
  window.removeEventListener('ldsshmanager:reconnect-session', onReconnectSession)
  offUnlocked?.()
})

// onReconnectSession 由 App 在"重置并重连"后广播: 按保存会话 id 找到配置,
// 用原标签 tempId 重新连接(不新开标签, connected 事件会替换该标签内容)。
async function onReconnectSession(e: Event) {
  const d = (e as CustomEvent).detail
  if (!d || !d.sessionId || !d.tempId) return
  const s = findSavedSession(d.sessionId)
  if (!s) return
  await runConnect(s, d.tempId, 80, 24)
}

// findSavedSession 按保存会话 id 在分组树中查找配置。
function findSavedSession(id: number): SavedSession | null {
  for (const g of state.groups) {
    const hit = g.sessions.find((x) => x.id === id)
    if (hit) return hit
  }
  return null
}

async function loadSessions() {
  let list
  try {
    list = await ListSavedSessions()
  } catch (err) {
    // 数据库未打开(启动锁定状态)时静默; 解锁后由 vault:unlocked 广播刷新
    console.error('load sessions:', err)
    state.groups = []
    return
  }
  const map = new Map<string, SavedSession[]>()
  for (const s of list || []) {
    if (!map.has(s.groupName)) map.set(s.groupName, [])
    map.get(s.groupName)!.push(s)
  }
  for (const sessions of map.values()) {
    sessions.sort((a, b) => a.name.localeCompare(b.name))
  }
  state.groups = Array.from(map.entries())
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([name, sessions]) => ({
      name,
      sessions,
      expanded: true, // 默认全展开; 持久化有记录时由 applyPersistedGroups 覆盖
    }))
  // 应用上次手动展开/折叠的状态(新增分组默认展开)
  await applyPersistedGroups()
}

// 分组展开/折叠持久化: 用单个 JSON key 存所有分组状态, 减少 GetSetting/SetSetting 调用
async function applyPersistedGroups() {
  try {
    const raw = await GetSetting('ui.treeGroupsExpanded')
    if (!raw) return
    const map = JSON.parse(raw) as Record<string, boolean>
    for (const g of state.groups) {
      if (typeof map[g.name] === 'boolean') g.expanded = map[g.name]
    }
  } catch {}
}

async function savePersistedGroups() {
  const map: Record<string, boolean> = {}
  for (const g of state.groups) map[g.name] = g.expanded
  try {
    await SetSetting('ui.treeGroupsExpanded', JSON.stringify(map))
  } catch {}
}

function toggleGroup(g: Group) {
  g.expanded = !g.expanded
  // 持久化: 记住用户最后手动展开/折叠的状态, 下次启动恢复
  savePersistedGroups()
}

// tryRestore 恢复上次打开的会话(有锁屏密码时启动为锁定态, DB 未打开会失败,
// 此时 restored 保持 false, 解锁后 vault:unlocked 会再次调用)。
async function tryRestore() {
  if (restored) return
  if (await restoreLastSessions()) restored = true
}

// restoreLastSessions reconnects the sessions that were open last time the app
// closed, in the same order. Password-typed sessions prompt for a password;
// skipping a prompt just skips that session.
// 返回 true = 已处理完成(无需重试); false = 数据库未打开等异常(需解锁后重试)。
async function restoreLastSessions(): Promise<boolean> {
  try {
    const raw = await GetSetting('ui.lastSessions')
    if (!raw) return true
    const ids: number[] = JSON.parse(raw)
    if (!Array.isArray(ids) || ids.length === 0) return true
    const byId = new Map<number, SavedSession>()
    for (const g of state.groups) {
      for (const s of g.sessions) byId.set(s.id, s)
    }
    for (const id of ids) {
      const s = byId.get(id)
      if (!s) continue
      await connectSession(s)
    }
    return true
  } catch (err: any) {
    // 数据库未打开(锁屏锁定态)等场景: 留待解锁后重试
    console.error('restore last sessions:', err)
    return false
  }
}

function resetForm() {
  state.form = {
    id: 0,
    name: '',
    host: '',
    port: 22,
    username: 'root',
    authType: 'password',
    keyPath: '',
    groupName: 'default',
    sortOrder: 0,
    password: '',
    proxyType: 'none',
    proxyHost: '',
    proxyPort: 1080,
    proxyUsername: '',
    proxyPassword: '',
    execCmd: '',
  }
}

function openAdd() {
  resetForm()
  state.formMode = 'add'
  state.showForm = true
}

function openEdit(s: SavedSession) {
  closeContextMenu()
  state.formMode = 'edit'
  state.form.id = s.id
  state.form.name = s.name
  state.form.host = s.host
  state.form.port = s.port
  state.form.username = s.username
  state.form.authType = (s.authType as 'password' | 'key') || 'password'
  state.form.keyPath = s.keyPath || ''
  state.form.groupName = s.groupName
  state.form.sortOrder = s.sortOrder
  state.form.proxyType = (s.proxyType as 'none' | 'socks5' | 'http') || 'none'
  state.form.proxyHost = s.proxyHost || ''
  state.form.proxyPort = s.proxyPort || 1080
  state.form.proxyUsername = s.proxyUsername || ''
  state.form.proxyPassword = ''
  state.form.execCmd = s.execCmd || ''
  state.form.password = s.password || ''
  state.showForm = true
}

async function saveSession() {
  const payload: SavedSession = {
    id: state.form.id,
    name: state.form.name,
    host: state.form.host,
    port: state.form.port,
    username: state.form.username,
    authType: state.form.authType,
    keyPath: state.form.keyPath,
    groupName: state.form.groupName,
    sortOrder: state.form.sortOrder,
    proxyType: state.form.proxyType,
    proxyHost: state.form.proxyHost,
    proxyPort: state.form.proxyPort,
    proxyUsername: state.form.proxyUsername,
    execCmd: state.form.execCmd,
    password: state.form.authType === 'password' ? state.form.password : '',
  }
  // SaveSession 必须在 try 内: 数据库未打开等场景会抛错,
  // 若不加 catch 事件被吞, 表现就是"点了保存没反应"。
  try {
    await SaveSession(payload)
  } catch (err: any) {
    await showToast(String(err), t('操作失败'), 5000, 'error')
    return
  }
  await loadSessions()
  state.showForm = false
  resetForm()
}

function connectSession(s: SavedSession) {
  selectedId.value = s.id
  // 立即打开标签(connecting 状态)并发起连接.
  // PTY 初始尺寸用后端默认 80x24(对齐开源 Web SSH 标准: 不在连接时传前端测量值,
  // 连接建立后由 TerminalPanel 的 term.onResize 事件自动同步真实尺寸,
  // 彻底消除 measureAndNotify + 兜底 setTimeout 的 race condition).
  const tempId = 'conn-' + Date.now().toString(36) + '-' + Math.random().toString(36).slice(2, 6)
  emit('open', {
    id: tempId,
    name: s.name,
    host: s.host,
    sessionId: s.id,
    status: 'connecting',
  })
  void runConnect(s, tempId, 80, 24)
}

async function runConnect(
  s: SavedSession,
  tempId: string,
  cols: number,
  rows: number,
  creds?: {password: string; proxyPassword: string},
) {
  // 重连(信任主机密钥后)使用已获取凭证, 避免重复弹密码/代理框; 否则用保存凭证或弹框。
  let password = creds?.password ?? s.password ?? ''
  if (s.authType === 'password' && !password) {
    const input = await showPrompt(t('密码认证'), '', t('连接'), t('连接'), t('取消'))
    if (input === null) {
      emit('failed', {tempId, error: t('取消')})
      return
    }
    password = input
  }
  let proxyPassword = creds?.proxyPassword ?? ''
  if (s.proxyType && s.proxyType !== 'none' && s.proxyUsername && !proxyPassword) {
    const input = await showPrompt(
      `${s.proxyUsername}@${s.proxyHost}`,
      '',
      t('代理设置'),
      t('连接'),
      t('取消')
    )
    if (input === null) {
      emit('failed', {tempId, error: t('取消')})
      return
    }
    proxyPassword = input
  }
  try {
    const result = await ConnectSSH({
      sessionId: s.id,
      password,
      keyPEM: '',
      keyPath: s.authType === 'key' ? s.keyPath : '',
      proxyPassword,
      cols,
      rows,
    })
    // 主机密钥未知/变更: 弹窗确认, 信任后带凭证重连
    if (result.hostKeyPrompt) {
      await handleHostKey(s, tempId, cols, rows, result.hostKeyPrompt, {password, proxyPassword})
      return
    }
    emit('connected', {
      tempId,
      session: {id: result.id, name: result.name, host: result.host, sessionId: result.sessionId, status: 'active'},
    })
  } catch (err: any) {
    emit('failed', {tempId, error: String(err)})
  }
}

// handleHostKey 处理"未知/变更主机密钥"弹窗: changed=中间人风险(阻止);
// unknown=用户确认后写入 known_hosts 并重连(带已获取凭证, 不再弹密码框)。
async function handleHostKey(
  s: SavedSession,
  tempId: string,
  cols: number,
  rows: number,
  prompt: any,
  creds: {password: string; proxyPassword: string},
): Promise<void> {
  if (prompt.changed) {
    await showToast(
      t('主机密钥已变更（{fp}），可能存在中间人攻击，连接已被阻止', {fp: prompt.fingerprint}),
      t('安全警告'),
      5000,
      'error',
    )
    emit('failed', {tempId, error: t('主机密钥已变更')})
    return
  }
  const ok = await showConfirm(
    t('未知主机 {host}（{type}）\n指纹: {fp}\n\n是否信任并继续连接？', {
      host: prompt.host,
      type: prompt.keyType,
      fp: prompt.fingerprint,
    }),
    t('信任并连接'),
    t('信任'),
    t('取消'),
  )
  if (!ok) {
    emit('failed', {tempId, error: t('已取消')})
    return
  }
  try {
    await TrustHostKey(prompt.host, prompt.keyType, prompt.blob)
  } catch (e: any) {
    await showToast(String(e), t('信任主机失败'), 5000, 'error')
    emit('failed', {tempId, error: String(e)})
    return
  }
  // 信任后带凭证重连(不再弹密码/代理框)
  await runConnect(s, tempId, cols, rows, creds)
}

async function duplicateSaved(s: SavedSession) {
  closeContextMenu()
  try {
    await DuplicateSavedSession(s.id)
    await loadSessions()
  } catch (err: any) {
    await showToast(String(err), t('复制会话'), 5000, 'error')
  }
}

// resetHostKey 清除该主机的 known_hosts 记录(远端重装/换密钥后使用),
// 清除后立即重新连接 → 走"未知主机"指纹确认流程(不绕过安全确认)。
async function resetHostKey(s: SavedSession) {
  closeContextMenu()
  const ok = await showConfirm(
    t('重置主机密钥 "{host}"？\n将删除已保存的主机密钥记录，下次连接将重新确认指纹。', {host: s.host}),
    t('重置主机密钥'),
    t('重置'),
    t('取消'),
  )
  if (!ok) return
  try {
    await ResetHostKey(s.host)
  } catch (err: any) {
    await showToast(String(err), t('操作失败'), 5000, 'error')
    return
  }
  await showToast(t('已重置主机密钥'), t('提示'), 3000, 'success')
  await connectSession(s)
}

async function removeSession(s: SavedSession) {
  closeContextMenu()
  const ok = await showConfirm(t('删除会话 "{name}"？', {name: s.name}), t('确认'), t('删除'), t('取消'))
  if (!ok) return
  await DeleteSession(s.id)
  await loadSessions()
}

function openContextMenu(e: MouseEvent, s: SavedSession) {
  contextMenu.x = e.clientX
  contextMenu.y = e.clientY
  contextMenu.session = s
  contextMenu.show = true
}

function closeContextMenu() {
  contextMenu.show = false
  contextMenu.session = null
}

async function browseKey() {
  const p = await PickFile()
  if (!p) return
  state.form.keyPath = p
}

function isActive(s: SavedSession): boolean {
  return false
}
</script>

<template>
  <aside class="session-tree">
    <header class="tree-header">
      <span>{{ t('会话') }}</span>
      <button class="icon-btn icon-btn-text" :title="t('新建会话')" @click="openAdd"><IconPark name="add" :size="12" :gap="2" /> {{ t('新建会话') }}</button>
    </header>

    <div class="tree-body">
      <div v-for="g in state.groups" :key="g.name" class="group">
        <div class="group-title" @click="toggleGroup(g)">
          <span class="arrow" :class="{collapsed: !g.expanded}"><IconPark name="right" :size="12" :gap="0" /></span>
          <span class="group-name">{{ g.name }} ({{ g.sessions.length }})</span>
        </div>
        <ul v-show="g.expanded" class="session-list">
          <li
            v-for="s in g.sessions"
            :key="s.id"
            class="session-item"
            :class="{active: isActive(s), 'session-item-selected': selectedId === s.id}"
            @click="selectSession(s)"
            @dblclick="connectSession(s)"
            @contextmenu.prevent="openContextMenu($event, s)"
          >
            <span class="session-dot" :class="{on: props.activeIds.includes(String(s.id))}"></span>
            <span class="session-name">{{ s.name }}</span>
          </li>
        </ul>
      </div>
      <div v-if="state.groups.length === 0" class="empty">{{ t('尚无会话') }}</div>
    </div>

    <div v-if="state.showForm" class="modal-overlay visible" @click.self="state.showForm = false">
      <div class="modal">
        <h3>{{ state.formMode === 'add' ? t('新建会话') : `${t('编辑会话')} · ${state.form.name}` }}</h3>

        <label class="field-label"><span>{{ t('名称') }}</span><input v-model="state.form.name" /></label>
        <label class="field-label"><span>{{ t('主机') }}</span><input v-model="state.form.host" /></label>
        <label class="field-label"><span>{{ t('端口') }}</span><input v-model.number="state.form.port" type="number" /></label>
        <label class="field-label"><span>{{ t('用户名') }}</span><input v-model="state.form.username" /></label>
        <label class="field-label"><span>{{ t('分组') }}</span><input v-model="state.form.groupName" /></label>

        <label class="field-label">
          <span>{{ t('认证方式') }}</span>
          <select v-model="state.form.authType">
            <option value="password">{{ t('密码认证') }}</option>
            <option value="key">{{ t('密钥认证') }}</option>
          </select>
        </label>
        <label v-if="state.form.authType === 'password'" class="field-label">
          <span>{{ t('密码') }}</span>
          <input v-model="state.form.password" type="password" @keyup.enter="saveSession" />
        </label>
        <label v-if="state.form.authType === 'key'" class="field-label key-row">
          <span>{{ t('密钥文件') }}</span>
          <input v-model="state.form.keyPath" readonly :placeholder="t('选择密钥文件')" />
          <button class="mini-btn" @click="browseKey()" type="button">…</button>
        </label>

        <label class="field-label">
          <span>{{ t('连接后执行') }}</span>
          <input v-model="state.form.execCmd" placeholder="ls -la" />
        </label>

        <details class="proxy-section" open>
          <summary>{{ t('代理设置') }}</summary>
          <label class="field-label">
            <span>{{ t('代理类型') }}</span>
            <select v-model="state.form.proxyType">
              <option value="none">{{ t('无') }}</option>
              <option value="socks5">SOCKS5</option>
              <option value="http">HTTP</option>
            </select>
          </label>
          <template v-if="state.form.proxyType !== 'none'">
            <label class="field-label"><span>{{ t('主机') }}</span><input v-model="state.form.proxyHost" /></label>
            <label class="field-label"><span>{{ t('端口') }}</span><input v-model.number="state.form.proxyPort" type="number" /></label>
            <label class="field-label"><span>{{ t('用户名') }}</span><input v-model="state.form.proxyUsername" /></label>
            <label class="field-label"><span>{{ t('密码') }}</span><input v-model="state.form.proxyPassword" type="password" /></label>
          </template>
        </details>

        <div class="modal-actions">
          <button class="btn-primary" @click="saveSession">{{ t('保存') }}</button>
          <button @click="state.showForm = false">{{ t('取消') }}</button>
        </div>
      </div>
    </div>

    <div v-if="contextMenu.show" class="context-overlay" @click="closeContextMenu"></div>
    <div
      v-if="contextMenu.show"
      class="context-menu"
      :style="{left: contextMenu.x + 'px', top: contextMenu.y + 'px'}"
      @click.stop
    >
      <div class="context-item" @click="openEdit(contextMenu.session!)">{{ t('编辑会话') }}</div>
      <div class="context-item" @click="duplicateSaved(contextMenu.session!)">{{ t('复制会话') }}</div>
      <div class="context-item" @click="resetHostKey(contextMenu.session!)">{{ t('重置主机密钥') }}</div>
      <div class="context-item context-danger" @click="removeSession(contextMenu.session!)">{{ t('删除会话') }}</div>
    </div>
  </aside>
</template>

<style scoped>
.session-tree {
  display: flex;
  flex-direction: column;
  width: 100%;
  min-width: 0;
  height: 100%;
  background: var(--panel);
  color: var(--text);
  border-right: 1px solid var(--border);
  user-select: none;
}
.tree-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  font-weight: 700;
  background: var(--panel-2);
  border-bottom: 1px solid var(--border);
}
.icon-btn {
  width: 22px;
  height: 22px;
  line-height: 20px;
  border: none;
  border-radius: 3px;
  background: var(--primary);
  color: #fff;
  cursor: pointer;
}
/* 图标+文字变体: 自适应宽度, 用于"新建会话"等头部按钮(与 Topbar 图标+文字风格一致) */
.icon-btn-text {
  width: auto;
  height: 22px;
  line-height: 1;
  padding: 0 8px;
  font-size: 12px;
  display: inline-flex;
  align-items: center;
}
.tree-body {
  flex: 1;
  overflow-y: auto;
  padding: 6px 0;
}
.group {
  margin-bottom: 4px;
}
.group-title {
  display: flex;
  align-items: center;
  padding: 5px 10px;
  cursor: pointer;
  color: var(--text-2);
}
.group-title:hover {
  background: var(--hover);
}
.arrow {
  display: inline-block;
  line-height: 0;
  margin-right: 6px;
  transition: transform 0.15s;
  vertical-align: middle;
}
.arrow.collapsed {
  transform: rotate(-90deg);
}
.session-list {
  list-style: none;
  margin: 0;
  padding: 0;
}
.session-item {
  display: flex;
  align-items: center;
  padding: 5px 10px 5px 26px;
  cursor: pointer;
  position: relative;
}
.session-item:hover,
.session-item.active {
  background: var(--hover-2);
}
.session-item-selected {
  background: var(--hover);
  box-shadow: inset 3px 0 0 var(--accent);
}
.session-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--text-3);
  margin-right: 8px;
}
.session-dot.on {
  background: var(--success);
}
.session-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.empty {
  padding: 20px;
  text-align: center;
  opacity: 0.5;
}
.modal-overlay {
  /* 与 style.css 的 .modal-overlay 一致: fixed 铺满视口 + z-index:2500,
     避免 scoped 的 z-index:100 被 lang-menu 等上层元素遮住(导致弹窗"看不见").
     注意: Vue scoped 会给类加 data-v 属性, 优先级高于 global, 因此这里必须显式声明
     与 global 相同的 fixed/高 z-index 才能真正盖在最上面。
     ⚠️ 全局 .modal-overlay 默认 opacity:0(配合 .visible 做淡入), scoped 未声明的
     属性仍会继承全局值 —— 因此这里必须显式 opacity:1(或模板加 .visible 类),
     否则弹窗渲染出来是全透明的, 表现就是"点击没反应"。*/
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 2500;
  opacity: 1;
}
.modal {
  background: var(--panel-2);
  padding: 18px;
  border-radius: 6px;
  width: 420px;
  max-width: 90vw;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
}
.modal h3 {
  margin: 0 0 14px;
}
.field-label {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}
.field-label > span {
  width: 76px;
  flex-shrink: 0;
  color: var(--text-2);
  text-align: right;
  white-space: nowrap;
}
.field-label input,
.field-label select {
  flex: 1;
  min-width: 0;
  box-sizing: border-box;
  padding: 5px 8px;
  border: 1px solid var(--border-2);
  border-radius: 3px;
  background: var(--panel);
  color: var(--input-text);
}
.modal-actions {
  display: flex;
  gap: 10px;
  margin-top: 14px;
}
.modal-actions button {
  flex: 1;
  padding: 7px;
  border: 1px solid var(--border-2);
  border-radius: 3px;
  background: var(--hover-2);
  color: var(--text);
  cursor: pointer;
}
.modal-actions .btn-primary {
  background: var(--primary);
  border-color: var(--primary);
  color: #fff;
}
.key-row input {
  flex: 1;
}
.mini-btn {
  flex-shrink: 0;
  width: 28px;
  padding: 5px 0;
  border: 1px solid var(--border-2);
  border-radius: 3px;
  background: var(--hover-2);
  color: var(--text);
  cursor: pointer;
}
.proxy-section {
  margin-top: 10px;
  padding-top: 8px;
  border-top: 1px solid var(--border);
}
.proxy-section summary {
  cursor: pointer;
  color: var(--text-2);
  margin-bottom: 8px;
  outline: none;
  user-select: none;
}
.proxy-section summary:hover {
  color: var(--text);
}
.context-overlay {
  position: fixed;
  inset: 0;
  z-index: 200;
}
.context-menu {
  position: fixed;
  z-index: 201;
  min-width: 140px;
  background: var(--panel-2);
  border: 1px solid var(--border-2);
  border-radius: 4px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
  padding: 4px 0;
}
.context-item {
  padding: 7px 14px;
  color: var(--text);
  cursor: pointer;
}
.context-item:hover {
  background: var(--hover-2);
}
.context-danger {
  color: var(--danger);
}
.context-danger:hover {
  background: var(--danger-hover);
}
</style>