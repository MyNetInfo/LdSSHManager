import {reactive} from 'vue'
import {t} from '../i18n'
import {
  VaultStatus,
  VaultSetPassword,
  VaultChangePassword,
  VaultDisablePassword,
  VaultUnlock,
  VaultLock,
} from '../../wailsjs/go/main/App'

// 锁屏状态(响应式, 供 Topbar / LockScreen 使用)
export const vaultState = reactive({
  enabled: false,
  locked: false,
  checked: false, // 是否已完成首次状态查询
})

// 锁屏密码管理弹窗(由 VaultModal.vue 渲染; Topbar / LockScreen 均可打开)
export const vaultModal = reactive({
  visible: false,
})

export function openVaultModal() {
  vaultModal.visible = true
}

export function closeVaultModal() {
  vaultModal.visible = false
}

// 把后端可能 reject 的调用统一成 { ok, ... } / { ok:false, error }
async function safe(p: Promise<any>): Promise<any> {
  try {
    return await p
  } catch (e: any) {
    return {ok: false, error: (e && e.message) || String(e)}
  }
}

// 初始化: 查询锁屏状态(启动时调用一次; 语言切换重挂载时重复调用无副作用)
export async function initVault(): Promise<void> {
  const r = await safe(VaultStatus())
  if (r && r.ok) {
    vaultState.enabled = !!r.enabled
    vaultState.locked = !!r.locked
  }
  vaultState.checked = true
}

// 顶部 🔒锁屏 按钮: 未设置密码 → 弹设置框(由组件处理); 已设置 → 立即锁定
export async function handleLockClick(): Promise<boolean> {
  const r = await safe(VaultStatus())
  if (r && r.ok) {
    vaultState.enabled = !!r.enabled
    vaultState.locked = !!r.locked
  }
  if (!vaultState.enabled) return false // 通知调用方打开设置框
  if (vaultState.locked) return true // 已锁定(理论上遮罩已覆盖)
  // 立即锁定
  await safe(VaultLock())
  vaultState.locked = true
  return true
}

export async function doUnlock(password: string): Promise<string | null> {
  const res = await safe(VaultUnlock(password))
  if (!res || !res.ok) {
    return (res && res.error) || '锁屏密码错误'
  }
  vaultState.locked = false
  return null
}

export async function doLock(): Promise<void> {
  await safe(VaultLock())
  vaultState.locked = true
}

export async function doSetPassword(pw: string): Promise<string | null> {
  const res = await safe(VaultSetPassword(pw))
  if (!res || !res.ok) return (res && res.error) || t('操作失败')
  vaultState.enabled = true
  vaultState.locked = false
  return null
}

export async function doChangePassword(oldPw: string, newPw: string): Promise<string | null> {
  const res = await safe(VaultChangePassword(oldPw, newPw))
  if (!res || !res.ok) return (res && res.error) || t('操作失败')
  vaultState.enabled = true
  vaultState.locked = false
  return null
}

export async function doDisablePassword(oldPw: string): Promise<string | null> {
  const res = await safe(VaultDisablePassword(oldPw))
  if (!res || !res.ok) return (res && res.error) || t('操作失败')
  vaultState.enabled = false
  vaultState.locked = false
  return null
}
