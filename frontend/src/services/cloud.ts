import {reactive} from 'vue'
import {t} from '../i18n'
import {
  CloudAuthGetVcode,
  CloudAuthLogin,
  CloudAuthRegister,
  CloudAuthLogout,
  CloudAuthStatus,
  CloudUploadConfig,
  CloudListVersions,
  CloudDownloadConfig,
  CloudApplyDownloaded,
  CloudAutoSyncIgnore,
  CloudAutoSyncDismiss,
  CloudAutoSyncRetry,
} from '../../wailsjs/go/main/App'
import {EventsOn} from '../../wailsjs/runtime/runtime'

// 云端同步前端服务: 登录/注册/登出 + 上传/下载 + 自动同步事件。
// 仅同步 session 文本(后端已保证不含 key 文件二进制)。

export interface CloudAuthInfo {
  loggedIn: boolean
  user_name: string
  nick_name: string
  avatar: string
  intro: string
  email: string
  mobile: string
  id: number
  role: number
  time_reg: number
  time_login: number
}

export const authState = reactive<CloudAuthInfo>({
  loggedIn: false,
  user_name: '',
  nick_name: '',
  avatar: '',
  intro: '',
  email: '',
  mobile: '',
  id: 0,
  role: 0,
  time_reg: 0,
  time_login: 0,
})

// 顶层状态栏的"账号状态"由 Topbar 通过回调刷新
let statusCb: ((st: CloudAuthInfo) => void) | null = null
export function setCloudStatusCallback(cb: (st: CloudAuthInfo) => void) {
  statusCb = cb
}
function updateStatus() {
  if (statusCb) statusCb({...authState})
}

// 把后端可能 reject 的调用统一成 { ok, ... } / { ok:false, error }
async function safe(p: Promise<any>): Promise<any> {
  try {
    return await p
  } catch (e: any) {
    return {ok: false, error: (e && e.message) || String(e)}
  }
}

let eventsBound = false

export async function initCloudSync(onTokenInvalid: () => void, onPasswordInvalid: (msg: string) => void) {
  await loadAuthStatus()
  if (eventsBound) return
  eventsBound = true

  // 自动同步发现 token 被其它设备顶掉
  EventsOn('auto-sync:token-invalid', () => {
    authState.loggedIn = false
    authState.user_name = ''
    authState.nick_name = ''
    updateStatus()
    if (onTokenInvalid) onTokenInvalid()
  })

  // 自动同步发现加密密码失效
  EventsOn('auto-sync:password-invalid', (msg: any) => {
    if (onPasswordInvalid) onPasswordInvalid(msg || '')
  })
}

async function loadAuthStatus() {
  try {
    const r = await CloudAuthStatus()
    if (r && r.ok) {
      authState.loggedIn = !!r.loggedIn
      authState.user_name = r.user_name || ''
      authState.nick_name = r.nick_name || ''
      authState.avatar = r.avatar || ''
      authState.intro = r.intro || ''
      authState.email = r.email || ''
      authState.mobile = r.mobile || ''
      authState.id = r.id || 0
      authState.role = r.role || 0
      authState.time_reg = r.time_reg || 0
      authState.time_login = r.time_login || 0
      updateStatus()
    }
  } catch (e) {
    /* ignore */
  }
}

// ---- 登录 / 登出 ----

export async function doAuthLogout(): Promise<string | null> {
  try {
    await CloudAuthLogout()
  } catch (e) {
    /* ignore */
  }
  authState.loggedIn = false
  authState.user_name = ''
  authState.nick_name = ''
  updateStatus()
  return null
}

export async function getVcode(): Promise<any> {
  return safe(CloudAuthGetVcode())
}

export async function doLogin(username: string, password: string, vcodeId: string, vcodeNum: string): Promise<string | null> {
  let r: any
  try {
    r = await CloudAuthLogin(username, password, vcodeId, vcodeNum)
  } catch (e: any) {
    return (e && e.message) || t('登录失败')
  }
  if (!r || !r.ok) {
    return (r && r.error) || t('登录失败')
  }
  authState.loggedIn = true
  authState.user_name = r.user_name || ''
  authState.nick_name = r.nick_name || ''
  authState.avatar = r.avatar || ''
  authState.intro = r.intro || ''
  authState.email = r.email || ''
  authState.mobile = r.mobile || ''
  authState.id = r.id || 0
  authState.role = r.role || 0
  authState.time_reg = r.time_reg || 0
  authState.time_login = r.time_login || 0
  updateStatus()
  return null
}

export async function doRegister(username: string, password: string, nick: string, vcodeId: string, vcodeNum: string): Promise<string | null> {
  try {
    await CloudAuthRegister(username, password, nick, vcodeId, vcodeNum)
  } catch (e: any) {
    return (e && e.message) || t('注册失败')
  }
  return await doLogin(username, password, vcodeId, vcodeNum)
}

// ---- 上传 / 下载 ----

export async function doUpload(password: string, reset: boolean, force: boolean): Promise<any> {
  return safe(CloudUploadConfig(password, reset, force))
}

export async function listVersions(): Promise<any> {
  return safe(CloudListVersions())
}

export async function doDownload(version: number, password: string): Promise<any> {
  return safe(CloudDownloadConfig(version, password))
}

export async function applyDownloaded(sessions: any[], quickCommands: any[]): Promise<any> {
  return safe(CloudApplyDownloaded({schema: 2, sessions: sessions || [], quickCommands: quickCommands || []} as any))
}

export async function autoSyncIgnore(): Promise<any> {
  return safe(CloudAutoSyncIgnore())
}

export async function autoSyncDismiss(): Promise<any> {
  return safe(CloudAutoSyncDismiss())
}

export async function autoSyncRetry(password: string): Promise<any> {
  return safe(CloudAutoSyncRetry(password))
}
