<script lang="ts" setup>
// 顶部工具栏 (布局/顺序/图标 与 LdTools 一致)。
//
// 左:  [品牌] [导出数据] [导入数据] [⬆上传云端] [⬇下载云端]
// 右:                                              [− +] [🔒锁屏] [☀浅色] [🌐Language] [👤账号] [访问官网]
//
// - 导出数据/导入数据: session JSON 全量覆盖
// - 上传/下载云端: 加密同步 session 文本(不含 key 文件二进制)
// - 锁屏: 锁屏密码保险库 (vault + SQLCipher DB 加密)
// - 账号 (右侧): 云端账号登录状态 — 登录后显示昵称, 未登录显示"未登录"
// - 主题按钮文字: 始终显示"当前主题" ("☀ 浅色" / "🌙 深色")

import {computed, onMounted, ref, watch} from 'vue'
import * as cloudSvc from '../services/cloud'
import {vaultState, initVault, handleLockClick, openVaultModal} from '../services/vault'
import {t, LANGS, currentLang, getLang, setLang, langLabel} from '../i18n'
import {TERMINAL_THEME_PRESETS} from '../terminalThemes'
import {currentTheme, toggleTheme, themeLabel, initTheme} from '../theme'
import {BrowserOpenURL} from '../../wailsjs/runtime/runtime'
import {ExportData, ImportData, ImportDataWithPassword, ApplyImport, GetVersion, GetSetting, SetSetting} from '../../wailsjs/go/main/App'
import {showToast, showConfirm, showPrompt} from '../dialog'
import {readFontScale, applyFontScale, fontStep} from '../font'
import IconPark from './IconPark.vue'

const OFFICIAL_URL = 'https://www.gxlidang.com'
const LATEST_URL = 'https://www.gxlidang.com/soft/ldsshmanager'

const emit = defineEmits<{
  (e: 'refresh-sessions'): void
}>()

const themeBtnText = computed(() => themeLabel(currentTheme.value))
const accountText = ref(t('未登录'))
const accountLoggedIn = ref(false)
const accountAvatar = ref('')
const showUserInfoModal = ref(false)
// 未登录态下语言切换时, 重新翻译"未登录"文字(ref(t(...)) 只在 setup 求值一次, 不响应 lang 变化)
watch(currentLang, () => {
  if (!accountLoggedIn.value) accountText.value = t('未登录')
})
const accountInitial = computed(() => {
  const txt = (accountText.value || '').trim()
  return txt ? txt[0].toUpperCase() : '?'
})
const fmt = (s: string) => s || t('未填写')
const fmtTime = (ts: number) => (ts ? new Date(ts * 1000).toLocaleString() : t('未填写'))

// 语言菜单
const langMenuOpen = ref(false)
const langMenuRef = ref<HTMLElement | null>(null)
const langMenuPos = ref({left: 0, top: 0})

// 锁屏相关
const lockBtnTitle = ref('')

// 系统设置
const showSettingsModal = ref(false)
const bottomBlankRows = ref(0)
const terminalTheme = ref('light')
const autoLockMinutes = ref(0)

async function openSettingsModal() {
  const v = await GetSetting('terminal.bottomBlankRows')
  bottomBlankRows.value = v ? Math.max(0, parseInt(v, 10) || 0) : 0
  const vt = await GetSetting('terminal.theme')
  terminalTheme.value = vt || 'light'
  const va = await GetSetting('autoLockMinutes')
  autoLockMinutes.value = va ? Math.max(0, parseInt(va, 10) || 0) : 0
  showSettingsModal.value = true
}

async function saveSettings() {
  await SetSetting('terminal.bottomBlankRows', String(bottomBlankRows.value))
  await SetSetting('terminal.theme', terminalTheme.value)
  await SetSetting('autoLockMinutes', String(autoLockMinutes.value))
  showSettingsModal.value = false
  // 通知 TerminalPanel / App.vue 刷新设置
  window.dispatchEvent(new CustomEvent('ldsshmanager:settings-changed', { detail: { key: 'terminal.bottomBlankRows' } }))
  window.dispatchEvent(new CustomEvent('ldsshmanager:settings-changed', { detail: { key: 'terminal.theme' } }))
  window.dispatchEvent(new CustomEvent('ldsshmanager:settings-changed', { detail: { key: 'autoLockMinutes' } }))
  showToast(t('已保存'), t('提示'), 5000, 'success')
}

// 云端登录弹窗
const showLoginModal = ref(false)
const loginMode = ref<'login' | 'register'>('login')
const loginUser = ref('')
const loginPass = ref('')
const loginNick = ref('')
const loginVcode = ref('')
const vcodeId = ref('')
const vcodeSrc = ref('')
const loginHint = ref('')

// 把后端返回的错误文本拆成「普通文本 + URL」片段, URL 渲染为可点击链接,
// 点击后通过 Wails BrowserOpenURL 在系统默认浏览器打开(不会离开 webview 应用)
function openExtLink(url: string) {
  if (!url) return
  BrowserOpenURL(url)
}
const URL_RE = /(https?:\/\/[^\s,，)）。;；]+)/g
const loginHintParts = computed(() => {
  const s = loginHint.value || ''
  if (!s) return [] as { text: string, url?: string }[]
  const parts: { text: string, url?: string }[] = []
  let last = 0
  URL_RE.lastIndex = 0
  let m: RegExpExecArray | null
  while ((m = URL_RE.exec(s)) !== null) {
    if (m.index > last) parts.push({ text: s.slice(last, m.index) })
    parts.push({ text: m[1], url: m[1] })
    last = m.index + m[1].length
  }
  if (last < s.length) parts.push({ text: s.slice(last) })
  return parts
})

// 上传/下载辅助
const showConflict = ref(false)
const showVersionPick = ref(false)
const versions = ref<any[]>([])
const pendingAction = ref<'upload' | 'download' | null>(null)
const version = ref('')

async function safe(p: Promise<any>): Promise<any> {
  try {
    return await p
  } catch (e: any) {
    return {ok: false, error: (e && e.message) || String(e)}
  }
}

onMounted(async () => {
  initTheme()
  applyFontScale()
  await initVault()
  refreshLockTitle()
  version.value = (await safe(GetVersion())) || ''
  cloudSvc.setCloudStatusCallback((st) => {
    if (st.loggedIn) {
      const name = (st.nick_name || st.user_name || '').trim()
      accountText.value = name || t('已登录')
      accountLoggedIn.value = true
      accountAvatar.value = st.avatar || ''
    } else {
      accountText.value = t('未登录')
      accountLoggedIn.value = false
      accountAvatar.value = ''
    }
  })
  cloudSvc.initCloudSync(
    () => {
      showToast(t('云端登录已在其它设备失效, 请重新登录'), t('提示'), 5000, 'error')
      openLoginModal()
    },
    (msg: string) => {
      showToast(msg || t('云端加密密码已失效, 自动同步已停止'), t('提示'), 5000, 'error')
    }
  )
})

function refreshLockTitle() {
  lockBtnTitle.value = vaultState.enabled
    ? t('已启用锁屏密码, 点击立即锁定')
    : t('未设置锁屏密码, 点击设置')
}

// ===========================================================================
// 导出 / 导入
// ===========================================================================

async function exportData() {
  // 导出前警告: 数据将用锁屏密码加密, 忘记密码无法恢复
  const warn = await showConfirm(
    t('导出数据将用锁屏密码加密, 忘记密码将无法恢复, 请务必牢记!'),
    t('导出数据'),
    t('继续导出'),
    t('取消')
  )
  if (!warn) return
  const r = await safe(ExportData())
  if (!r || !r.ok) {
    if (r && r.canceled) return
    showToast((r && r.error) || t('导出失败'), t('错误'), 5000, 'error')
    return
  }
  if (r.usedDefault) {
    showToast(
      t('已导出数据: 会话 {n} 条, 快捷命令 {m} 条 (未设锁屏密码, 用默认密码 123456 加密)', {n: r.count || 0, m: r.qcCount || 0}),
      t('提示'), 5000, 'info'
    )
  } else {
    showToast(
      t('已导出数据: 会话 {n} 条, 快捷命令 {m} 条 (已用锁屏密码加密, 导入需输该密码)', {n: r.count || 0, m: r.qcCount || 0}),
      t('提示'), 5000, 'info'
    )
  }
}

async function importData() {
  const r = await safe(ImportData())
  if (!r || !r.ok) {
    if (r && r.canceled) return
    showToast((r && r.error) || t('导入失败'), t('错误'), 5000, 'error')
    return
  }
  // 默认密码 123456 解不开 → 弹密码框让用户输入备份时的锁屏密码
  if (r.needPassword) {
    const pw = await showPrompt(
      t('该备份不是用默认密码加密, 请输入备份时使用的锁屏密码来解密:'),
      '',
      t('输入备份密码'),
      t('解密并导入'),
      t('取消'),
      true
    )
    if (pw == null || !pw) return
    const rr = await safe(ImportDataWithPassword(r.filePath, pw))
    if (!rr || !rr.ok) {
      showToast((rr && rr.error) || t('密码错误, 无法解密'), t('错误'), 5000, 'error')
      return
    }
    await confirmAndApply(rr)
    return
  }
  await confirmAndApply(r)
}

// 导入确认后执行覆盖(会话 + 快捷命令), 并刷新会话树与左右快捷命令面板。
async function confirmAndApply(r: any) {
  const ok = await showConfirm(
    t('确定用此备份覆盖本地 (会话 {n} 条, 快捷命令 {m} 条)?', {n: r.count || 0, m: r.qcCount || 0}),
    t('导入数据'),
    t('导入'),
    t('取消')
  )
  if (!ok) return
  const rr = await safe(ApplyImport(r.sessions || [], r.quickCommands || []))
  if (!rr || !rr.ok) {
    showToast((rr && rr.error) || t('导入失败'), t('错误'), 5000, 'error')
    return
  }
  showToast(
    t('已导入数据 (会话 {n} 条, 快捷命令 {m} 条)', {n: rr.count || 0, m: rr.qcCount || 0}),
    t('提示'), 5000, 'success'
  )
  emit('refresh-sessions')
  // 左右快捷命令面板通过跨组件事件联动刷新
  window.dispatchEvent(new Event('ldsshmanager:refresh-quick-commands'))
}

// ===========================================================================
// 主题 / 字号
// ===========================================================================

function onToggleTheme() {
  toggleTheme()
  // themeBtnText 为 computed, 自动更新
}

// ===========================================================================
// 语言菜单
// ===========================================================================

function openLangMenu(e: MouseEvent) {
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
  langMenuPos.value = {left: r.left, top: r.bottom + 6}
  langMenuOpen.value = true
}

function pickLang(code: string) {
  setLang(code)
  langMenuOpen.value = false
  showToast(t('语言: {label}', {label: langLabel(code)}), t('提示'), 5000, 'info')
}

// ===========================================================================
// 锁屏
// ===========================================================================

async function onLockClick() {
  const locked = await handleLockClick()
  refreshLockTitle()
  if (locked) return // 已锁定或已锁定成功 → 锁屏遮罩显示
  // 未设置密码: 打开锁屏密码设置弹窗
  openVaultModal()
}

// ===========================================================================
// 云端账号
// ===========================================================================

function openLoginModal() {
  loginMode.value = 'login'
  loginUser.value = ''
  loginPass.value = ''
  loginNick.value = ''
  loginVcode.value = ''
  vcodeId.value = ''
  loginHint.value = ''
  showLoginModal.value = true
  loadVcode()
}

async function loadVcode() {
  const r = await cloudSvc.getVcode()
  if (r && typeof r === 'object' && 'vcode' in r && r.vcode) {
    const raw = String(r.vcode).trim()
    vcodeSrc.value = raw.indexOf('data:') === 0 ? raw : 'data:image/png;base64,' + raw
    vcodeId.value = r.id != null ? String(r.id) : ''
  } else {
    vcodeSrc.value = ''
    vcodeId.value = ''
  }
}

function onAuthToggle() {
  if (accountLoggedIn.value) {
    showUserInfoModal.value = true
  } else {
    openLoginModal()
  }
}

async function onLogout() {
  showUserInfoModal.value = false
  await cloudSvc.doAuthLogout()
  showToast(t('已退出云端账号'), t('提示'), 5000, 'info')
}

async function doLoginOrRegister() {
  const user = loginUser.value.trim()
  const pass = loginPass.value
  if (!user || !pass) {
    loginHint.value = t('请输入用户名和密码')
    return
  }
  if (loginMode.value === 'register' && pass.length < 6) {
    loginHint.value = t('密码至少 6 位')
    return
  }
  loginHint.value = t('请稍候...')
  const err = loginMode.value === 'register'
    ? await cloudSvc.doRegister(user, pass, loginNick.value.trim(), vcodeId.value, loginVcode.value.trim())
    : await cloudSvc.doLogin(user, pass, vcodeId.value, loginVcode.value.trim())
  if (err) {
    loginHint.value = err
    loadVcode()
    loginVcode.value = ''
    return
  }
  showLoginModal.value = false
  showToast(t('登录成功'), t('提示'), 5000, 'success')
  // 登录成功后若之前是上传/下载意图, 继续执行
  const act = pendingAction.value
  pendingAction.value = null
  if (act === 'upload') doUploadFlow()
  else if (act === 'download') doDownloadFlow()
}

// ===========================================================================
// 上传 / 下载
// ===========================================================================

async function doUploadFlow() {
  if (!cloudSvc.authState.loggedIn) {
    showToast(t('请先登录云端账号'), t('提示'), 5000, 'info')
    pendingAction.value = 'upload'
    openLoginModal()
    return
  }
  let r = await cloudSvc.doUpload('', false, false)
  // 需要输入密码的流程: 循环直到成功/取消
  while (r && r.ok === false && r.needPassword) {
    // resetOption: 输入的密码解不开云端历史版本(可能已在其他电脑更换密码)。
    // 继续循环只会无限弹密码框 → 停下, 询问是否用新密码清除历史重传。
    if (r.resetOption) {
      const reset = await showConfirm(
        (r.error && t('{msg}, 是否用新密码清除云端历史后重新上传?', {msg: r.error})) ||
          t('该密码无法解密云端历史版本, 是否用新密码清除历史后重新上传?'),
        t('提示'),
        t('用新密码重新上传'),
        t('取消')
      )
      if (!reset) {
        showToast((r && r.error) || t('上传失败'), t('错误'), 5000, 'error')
        return
      }
      const pw = await promptSyncPassword(t('设置云端加密密码 (至少 6 位)'))
      if (pw == null || !pw) return
      r = await cloudSvc.doUpload(pw, true, false)
      break
    }
    const pw = await promptSyncPassword(t('设置云端加密密码 (至少 6 位)'))
    if (pw == null || !pw) return
    r = await cloudSvc.doUpload(pw, false, false)
  }
  if (!r || !r.ok) {
    showToast((r && r.error) || t('上传失败'), t('错误'), 5000, 'error')
    return
  }
  if (r.conflict) {
    showToast(t('检测到云端 session 与本地不一致, 请选择如何处理:'), t('提示'), 5000, 'info')
    showConflictDialog('upload')
    return
  }
  showToast(
    r.skipped ? t('云端已是最新, 无需上传') : r.reset ? t('已用新密码上传') : t('已加密上传到云端'),
    t('提示'),
    5000,
    r.skipped ? 'info' : 'success'
  )
}

async function doDownloadFlow() {
  if (!cloudSvc.authState.loggedIn) {
    showToast(t('请先登录云端账号'), t('提示'), 5000, 'info')
    pendingAction.value = 'download'
    openLoginModal()
    return
  }
  const vl = await cloudSvc.listVersions()
  if (!vl || !vl.ok) {
    showToast((vl && vl.error) || t('获取版本失败'), t('错误'), 5000, 'error')
    return
  }
  if (!vl.versions || !vl.versions.length) {
    showToast(t('云端没有已上传的配置'), t('错误'), 5000, 'error')
    return
  }
  versions.value = vl.versions
  showVersionPick.value = true
}

async function pickVersion(version: number) {
  showVersionPick.value = false
  let r = await cloudSvc.doDownload(version, '')
  while (r && r.ok === false && r.needPassword) {
    if (r.undecryptable) {
      showToast(t('无法解密云端数据'), t('错误'), 5000, 'error')
      return
    }
    const pw = await promptSyncPassword(t('输入云端加密密码'))
    if (pw == null || !pw) return
    r = await cloudSvc.doDownload(version, pw)
  }
  if (!r || !r.ok) {
    showToast((r && r.error) || t('下载失败'), t('错误'), 5000, 'error')
    return
  }
  if (r.same) {
    showToast(t('本地已是最新, 无需下载'), t('提示'), 5000, 'info')
    return
  }
  // 与本地不一致: 弹冲突选择
  pendingRemote = r
  showConflictDialog('download')
}

let pendingRemote: any = null

function showConflictDialog(kind: 'upload' | 'download') {
  showConflict.value = true
  conflictKind.value = kind
}

const conflictKind = ref<'upload' | 'download'>('upload')

async function conflictRemote() {
  showConflict.value = false
  // 用云端覆盖本地
  if (pendingRemote) {
    const rr = await cloudSvc.applyDownloaded(pendingRemote.sessions || [], pendingRemote.quickCommands || [])
    if (rr && rr.ok) {
      showToast(t('已用云端覆盖本地 (会话 {n} 条, 快捷命令 {m} 条)', {n: rr.count || 0, m: rr.qcCount || 0}), t('提示'), 5000, 'success')
      emit('refresh-sessions')
    } else {
      showToast((rr && rr.error) || t('应用失败'), t('错误'), 5000, 'error')
    }
    pendingRemote = null
    return
  }
  // 上传冲突时"用云端覆盖本地": 需先下载
  let r = await cloudSvc.doDownload(0, '')
  while (r && r.ok === false && r.needPassword) {
    const pw = await promptSyncPassword(t('输入云端加密密码'))
    if (pw == null || !pw) return
    r = await cloudSvc.doDownload(0, pw)
  }
  if (!r || !r.ok) {
    showToast((r && r.error) || t('下载失败'), t('错误'), 5000, 'error')
    return
  }
  const rr = await cloudSvc.applyDownloaded(r.sessions || [], r.quickCommands || [])
  if (rr && rr.ok) {
    showToast(t('已用云端覆盖本地 (会话 {n} 条, 快捷命令 {m} 条)', {n: rr.count || 0, m: rr.qcCount || 0}), t('提示'), 5000, 'success')
    emit('refresh-sessions')
  }
}

async function conflictLocal() {
  showConflict.value = false
  // 用本地覆盖云端
  let r = await cloudSvc.doUpload('', false, true)
  if (r && r.ok === false && r.needPassword) {
    const pw = await promptSyncPassword(t('输入云端加密密码'))
    if (pw == null || !pw) return
    r = await cloudSvc.doUpload(pw, false, true)
  }
  if (!r || !r.ok) {
    showToast((r && r.error) || t('上传失败'), t('错误'), 5000, 'error')
    return
  }
  showToast(t('已用本地覆盖云端'), t('提示'), 5000, 'success')
}

// 密码输入(取消返回 null)
function promptSyncPassword(title: string): Promise<string | null> {
  return new Promise((resolve) => {
    syncPwTitle.value = title
    syncPwValue.value = ''
    showSyncPw.value = true
    syncPwResolve = resolve
  })
}

const showSyncPw = ref(false)
const syncPwTitle = ref('')
const syncPwValue = ref('')
let syncPwResolve: ((v: string | null) => void) | null = null

function syncPwOk() {
  const v = syncPwValue.value
  showSyncPw.value = false
  if (syncPwResolve) syncPwResolve(v)
  syncPwResolve = null
}

function syncPwCancel() {
  showSyncPw.value = false
  if (syncPwResolve) syncPwResolve(null)
  syncPwResolve = null
}

// ===========================================================================
// 访问官网
// ===========================================================================

function openOfficial() {
  try {
    BrowserOpenURL(OFFICIAL_URL)
  } catch (e) {
    /* ignore */
  }
}

function openLatest() {
  try {
    BrowserOpenURL(LATEST_URL)
  } catch (e) {
    /* ignore */
  }
}</script>

<template>
  <header class="topbar">
    <!-- 左: 品牌 -->
    <div class="topbar-brand">
      <img class="brand-logo-img" src="../assets/icon.png" alt="LdSSHManager" />
      <h1 class="brand-title">LdSSHManager <span class="brand-version">{{ version }}</span></h1>
    </div>

    <!-- 左: 数据/云端 按钮簇 (紧贴品牌) -->
    <div class="topbar-cluster topbar-left">
      <button class="topbar-btn" :title="t('系统设置')" @click="openSettingsModal"><IconPark name="setting" /> {{ t('系统设置') }}</button>
      <button class="topbar-btn" :title="t('导出数据')" @click="exportData"><IconPark name="export" /> {{ t('导出数据') }}</button>
      <button class="topbar-btn" :title="t('导入数据')" @click="importData"><IconPark name="import" /> {{ t('导入数据') }}</button>
      <button class="topbar-btn" :title="t('加密上传 session 到云端')" @click="doUploadFlow"><IconPark name="upload" /> {{ t('上传云端') }}</button>
      <button class="topbar-btn" :title="t('从云端下载 session')" @click="doDownloadFlow"><IconPark name="download" /> {{ t('下载云端') }}</button>
    </div>

    <!-- 中间弹性空白 -->
    <div class="topbar-grow"></div>

    <!-- 右: 字号 / 锁屏 / 主题 / 语言 / 账号 / 访问官网 -->
    <div class="topbar-cluster">
      <!-- 字号 − / + -->
      <div class="font-ctrl">
        <button class="topbar-btn topbar-btn-icon" :title="t('缩小字号')" @click="fontStep(-0.1)"><IconPark name="minus" :size="14" :gap="0" /></button>
        <button class="topbar-btn topbar-btn-icon" :title="t('放大字号')" @click="fontStep(0.1)"><IconPark name="plus" :size="14" :gap="0" /></button>
      </div>

      <!-- 锁屏 -->
      <button class="topbar-btn" :title="lockBtnTitle" @click="onLockClick"><IconPark name="lock" /> {{ t('锁屏') }}</button>

      <!-- 主题 (按钮文字随当前主题动态) -->
      <button class="topbar-btn" :title="t('切换浅色 / 深色主题')" @click="onToggleTheme"><IconPark :name="currentTheme === 'light' ? 'sun' : 'moon'" /> {{ themeBtnText }}</button>

      <!-- 语言 -->
      <button class="topbar-btn" :title="t('切换界面语言')" @click="openLangMenu"><IconPark name="globe" /> Language</button>

      <!-- 账号状态 (云端) -->
      <button
        class="topbar-btn topbar-btn-account"
        :class="{'is-logged-in': accountLoggedIn}"
        :title="accountLoggedIn ? t('点击查看用户信息') : t('点击登录')"
        @click="onAuthToggle"
      >
        <img v-if="accountLoggedIn && accountAvatar" :src="accountAvatar" class="topbar-avatar" @error="accountAvatar=''" alt="" />
        <span v-else-if="accountLoggedIn" class="topbar-avatar topbar-avatar-fallback">{{ accountInitial }}</span>
        <IconPark v-else name="user" :size="16" :gap="3" />
        <span class="topbar-account-name">{{ accountText }}</span>
      </button>

      <!-- 查看最新版本 -->
      <button class="topbar-btn" :title="t('查看最新版本')" @click="openLatest"><IconPark name="link" /> {{ t('查看最新版本') }}</button>

      <!-- 访问官网 -->
      <button class="topbar-btn" :title="t('访问官网')" @click="openOfficial"><IconPark name="web" /> {{ t('访问官网') }}</button>
    </div>

    <!-- 语言菜单 -->
    <div
      v-if="langMenuOpen"
      ref="langMenuRef"
      class="lang-menu"
      :style="{left: langMenuPos.left + 'px', top: langMenuPos.top + 'px'}"
    >
      <div
        v-for="[code, label] in LANGS"
        :key="code"
        class="lang-menu-item"
        @click="pickLang(code)"
      >{{ label }}<IconPark v-if="getLang() === code" name="correct" :size="13" :gap="0" /></div>
    </div>

    <!-- 云端登录弹窗 -->
    <div v-if="showLoginModal" class="modal-overlay visible" @click.self="showLoginModal = false">
      <div class="modal-dialog">
        <div class="modal-header">
          <h3 class="modal-title">{{ t('云端账号登录') }}</h3>
          <button class="modal-close" @click="showLoginModal = false"><IconPark name="close" :size="14" :gap="0" /></button>
        </div>
        <div class="modal-body">
          <div class="form-row">
            <label class="form-label">{{ t('用户名') }}</label>
            <input v-model="loginUser" type="text" class="form-input" :placeholder="t('请输入用户名')" />
          </div>
          <div class="form-row">
            <label class="form-label">{{ t('密码') }}</label>
            <input v-model="loginPass" type="text" class="form-input mask-pw" :placeholder="t('请输入密码 (至少 6 位)')" />
          </div>
          <div v-if="loginMode === 'register'" class="form-row">
            <label class="form-label">{{ t('昵称') }}</label>
            <input v-model="loginNick" type="text" class="form-input" :placeholder="t('注册时填写, 可留空')" />
          </div>
          <div class="form-row">
            <label class="form-label">{{ t('验证码') }}</label>
            <input v-model="loginVcode" type="text" class="form-input is-fixed" style="width:90px" :placeholder="t('请输入验证码')" />
            <img v-if="vcodeSrc" class="vcode-img" :src="vcodeSrc" :title="t('验证码')" @click="loadVcode" />
            <img v-else class="vcode-img" :title="t('验证码')" @click="loadVcode" />
          </div>
          <p class="dialog-desc">{{ t('登录后可同步数据到云端, 上传到云端的数据会用 "同步密码" 加密后上传, 数据安全由你掌握!') }}</p>
          <p class="dialog-hint">
            <template v-for="(p, i) in loginHintParts" :key="i"><a v-if="p.url" href="javascript:void(0)" class="ext-link" @click.prevent="openExtLink(p.url)">{{ p.text }}</a><span v-else>{{ p.text }}</span></template>
          </p>
        </div>
        <div class="modal-actions">
          <button class="btn btn-small" @click="loginMode = loginMode === 'login' ? 'register' : 'login'">
            {{ loginMode === 'login' ? t('注册账号') : t('去登录') }}
          </button>
          <button class="btn btn-default" @click="showLoginModal = false">{{ t('取消') }}</button>
          <button class="btn btn-primary" @click="doLoginOrRegister">
            {{ loginMode === 'login' ? t('登录') : t('注册并登录') }}
          </button>
        </div>
      </div>
    </div>

    <!-- 用户信息弹窗 (点击已登录账号头像) -->
    <div v-if="showUserInfoModal" class="modal-overlay visible" @click.self="showUserInfoModal = false">
      <div class="modal-dialog account-modal-dialog">
        <div class="modal-header">
          <h3 class="modal-title">{{ t('用户信息') }}</h3>
          <button class="modal-close" @click="showUserInfoModal = false"><IconPark name="close" :size="14" :gap="0" /></button>
        </div>
        <div class="modal-body">
          <div class="account-modal">
            <img v-if="accountAvatar" :src="accountAvatar" class="account-avatar-lg" alt="" />
            <div v-else class="account-avatar-lg account-avatar-fallback-lg">{{ accountInitial }}</div>
            <p class="account-nick">{{ fmt(cloudSvc.authState.nick_name) }}</p>
            <p class="account-field">{{ t('用户名') }}: {{ fmt(cloudSvc.authState.user_name) }}</p>
            <p class="account-field">{{ t('用户ID') }}: {{ cloudSvc.authState.id || t('未填写') }}</p>
            <p class="account-field">{{ t('邮箱') }}: {{ fmt(cloudSvc.authState.email) }}</p>
            <p class="account-field">{{ t('手机') }}: {{ fmt(cloudSvc.authState.mobile) }}</p>
            <p class="account-field">{{ t('注册时间') }}: {{ fmtTime(cloudSvc.authState.time_reg) }}</p>
            <p class="account-field">{{ t('最近登录') }}: {{ fmtTime(cloudSvc.authState.time_login) }}</p>
            <p class="account-intro">{{ fmt(cloudSvc.authState.intro) }}</p>
          </div>
        </div>
        <div class="modal-actions">
          <button class="btn btn-default" @click="showUserInfoModal = false">{{ t('关闭') }}</button>
          <button class="btn btn-primary" @click="onLogout">{{ t('退出登录') }}</button>
        </div>
      </div>
    </div>

    <!-- 云端版本选择弹窗 -->
    <div v-if="showVersionPick" class="modal-overlay visible" @click.self="showVersionPick = false">
      <div class="modal-dialog">
        <div class="modal-header">
          <h3 class="modal-title">{{ t('选择云端版本') }}</h3>
          <button class="modal-close" @click="showVersionPick = false"><IconPark name="close" :size="14" :gap="0" /></button>
        </div>
        <div class="modal-body">
          <div class="dialog-list">
            <div
              v-for="(v, i) in versions"
              :key="v.version"
              class="dialog-list-item"
              @click="pickVersion(v.version)"
            >
              <span>{{ t('版本 {n}', {n: v.version}) }}{{ i === 0 ? ' (' + t('最新') + ')' : '' }}</span>
              <span class="dialog-list-time">{{ v.time_create ? new Date(Number(v.time_create) * 1000).toLocaleString() : '' }}</span>
            </div>
          </div>
        </div>
        <div class="modal-actions">
          <button class="btn btn-default" @click="showVersionPick = false">{{ t('取消') }}</button>
        </div>
      </div>
    </div>

    <!-- 冲突弹窗 -->
    <div v-if="showConflict" class="modal-overlay visible" @click.self="showConflict = false">
      <div class="modal-dialog">
        <div class="modal-header">
          <h3 class="modal-title">{{ t('云端与本地不一致') }}</h3>
          <button class="modal-close" @click="showConflict = false"><IconPark name="close" :size="14" :gap="0" /></button>
        </div>
        <div class="modal-body">
          <p class="dialog-desc">{{ t('检测到云端 session 与本地不一致, 请选择如何处理:') }}</p>
        </div>
        <div class="modal-actions">
          <button class="btn btn-small" @click="showConflict = false">{{ t('暂不处理') }}</button>
          <button class="btn btn-default" @click="conflictLocal">{{ t('用本地覆盖云端') }}</button>
          <button class="btn btn-primary" @click="conflictRemote">{{ t('用云端覆盖本地') }}</button>
        </div>
      </div>
    </div>

    <!-- 系统设置弹窗 -->
    <div v-if="showSettingsModal" class="modal-overlay visible" @click.self="showSettingsModal = false">
      <div class="modal-dialog" style="max-width:945px;text-align:left">
        <div class="modal-header">
          <h3 class="modal-title">{{ t('系统设置') }}</h3>
          <button class="modal-close" @click="showSettingsModal = false"><IconPark name="close" :size="14" :gap="0" /></button>
        </div>
        <div class="modal-body">
          <!-- 行 1: 终端风格 -->
          <div class="form-row">
            <label class="form-label">{{ t('终端风格') }}</label>
            <select v-model="terminalTheme" class="form-input" style="width:auto;min-width:180px">
              <option v-for="p in TERMINAL_THEME_PRESETS" :key="p.value" :value="p.value">{{ t(p.name) }}</option>
            </select>
          </div>
          <!-- 行 2: 自动锁屏 (固定在第 2 行, 将来加新行请插在它之后或最末, 勿插在前面) -->
          <div class="form-row" style="margin-top:10px">
            <label class="form-label">{{ t('自动锁屏') }}</label>
            <input v-model.number="autoLockMinutes" type="number" min="0" max="1440" class="form-input" style="width:80px" />
            <span class="form-hint" style="margin-left:8px;color:var(--text-muted)">{{ t('分钟 (0=不自动锁)') }}</span>
          </div>
          <!-- 行 3: 终端底部留空行数 -->
          <div class="form-row" style="margin-top:10px">
            <label class="form-label">{{ t('终端底部留空行数') }}</label>
            <input v-model.number="bottomBlankRows" type="number" min="0" max="20" class="form-input" style="width:80px" />
            <span class="form-hint" style="margin-left:8px;color:var(--text-muted)">{{ t('行 (0=不预留)') }}</span>
          </div>
        </div>
        <div class="modal-actions">
          <button class="btn btn-default" @click="showSettingsModal = false">{{ t('取消') }}</button>
          <button class="btn btn-primary" @click="saveSettings">{{ t('确定') }}</button>
        </div>
      </div>
    </div>

    <!-- 同步密码输入弹窗 -->
    <div v-if="showSyncPw" class="modal-overlay visible" @click.self="syncPwCancel">
      <div class="modal-dialog">
        <div class="modal-header">
          <h3 class="modal-title">{{ syncPwTitle }}</h3>
          <button class="modal-close" @click="syncPwCancel"><IconPark name="close" :size="14" :gap="0" /></button>
        </div>
        <div class="modal-body">
          <div class="form-row">
            <label class="form-label">{{ t('加密密码') }}</label>
            <input v-model="syncPwValue" type="text" class="form-input mask-pw" :placeholder="t('至少 6 位')" @keyup.enter="syncPwOk" />
          </div>
        </div>
        <div class="modal-actions">
          <button class="btn btn-default" @click="syncPwCancel">{{ t('取消') }}</button>
          <button class="btn btn-primary" @click="syncPwOk">{{ t('确定') }}</button>
        </div>
      </div>
    </div>
  </header>
</template>
