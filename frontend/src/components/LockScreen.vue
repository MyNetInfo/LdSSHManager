<script lang="ts" setup>
// 全屏锁屏遮罩: vaultState.locked 为 true 时显示, 输入密码解锁。
// 挂载在 App.vue 顶层, 与 LdTools 的锁屏遮罩行为一致。
// 遮罩上提供「锁屏密码管理」入口: 打开 VaultModal(修改/取消密码),
// 关闭后若仍未解锁则恢复遮罩。

import {onMounted, onUnmounted, ref} from 'vue'
import {vaultState, vaultModal, doUnlock, openVaultModal} from '../services/vault'
import {t} from '../i18n'
import {showToast} from '../dialog'
import {EventsOn} from '../../wailsjs/runtime/runtime'

const pw = ref('')
const err = ref('')

// 解锁成功后后端会 emit vault:unlocked(数据库已打开), 这里广播让会话树刷新
let offUnlocked: (() => void) | null = null
onMounted(() => {
  offUnlocked = EventsOn('vault:unlocked', () => {
    window.dispatchEvent(new CustomEvent('ldsshmanager:refresh-sessions'))
  })
})
onUnmounted(() => {
  offUnlocked?.()
})

async function unlock() {
  if (!pw.value) {
    err.value = t('请输入锁屏密码')
    return
  }
  const e = await doUnlock(pw.value)
  if (e) {
    err.value = e
    pw.value = ''
    return
  }
  err.value = ''
  pw.value = ''
  showToast(t('解锁成功'), t('提示'), 5000, 'success')
  // 兜底: 即使事件未到达也刷新一次
  window.dispatchEvent(new CustomEvent('ldsshmanager:refresh-sessions'))
}

// 打开锁屏密码管理弹窗(修改/取消密码)
function openManage() {
  openVaultModal()
}
</script>

<template>
  <!-- 打开锁屏密码管理弹窗时暂时隐藏遮罩, 关闭后若仍未解锁则恢复 -->
  <div v-if="vaultState.locked && !vaultModal.visible" class="lock-screen">
    <div class="lock-box">
      <div class="lock-logo">LdSSHManager</div>
      <div class="lock-sub">{{ t('已启用锁屏密码, 请输入密码解锁') }}</div>
      <input
        v-model="pw"
        type="password"
        class="lock-input"
        :placeholder="t('输入锁屏密码')"
        autocomplete="off"
        @keyup.enter="unlock"
      />
      <div class="lock-err">{{ err }}</div>
      <button class="lock-btn" @click="unlock">{{ t('解锁') }}</button>
      <a class="lock-manage" href="javascript:void(0)" @click="openManage">{{ t('锁屏密码管理') }}</a>
    </div>
  </div>
</template>
