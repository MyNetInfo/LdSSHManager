<script lang="ts" setup>
// 锁屏密码管理弹窗 (对齐 LdTools vault.js openVaultModal)。
// 三种用途: 未设置密码 → 设置; 已设置密码 → 修改 / 取消。
// 由 vault 服务统一控制开关(vaultModal.visible), Topbar 与 LockScreen 都可打开。

import {ref, watch} from 'vue'
import {vaultModal, vaultState, doSetPassword, doChangePassword, doDisablePassword} from '../services/vault'
import {t} from '../i18n'
import {showToast, showConfirm} from '../dialog'
import IconPark from './IconPark.vue'

const oldPw = ref('')
const newPw = ref('')
const confirmPw = ref('')
const hint = ref('')

// 每次打开时清空表单
watch(
  () => vaultModal.visible,
  (v) => {
    if (v) {
      oldPw.value = ''
      newPw.value = ''
      confirmPw.value = ''
      hint.value = ''
    }
  }
)

function close() {
  vaultModal.visible = false
}

async function save() {
  const np = newPw.value
  if (np.length < 6 || np.length > 64) {
    hint.value = t('密码长度需为 6-64 位')
    return
  }
  // 禁止使用默认密码 123456: 它是"未设置"状态的隐蔽密码(用户不知道有密码),
  // 设为它会导致锁屏要求输入用户不知道的密码, 且默认密码公开毫无保护意义。
  if (np === '123456') {
    hint.value = t('不能使用默认密码 123456, 请换一个')
    return
  }
  // 密码必须同时包含字母和数字(其它字符随意)
  if (!/[a-zA-Z]/.test(np) || !/\d/.test(np)) {
    hint.value = t('密码必须同时包含字母和数字')
    return
  }
  if (np !== confirmPw.value) {
    hint.value = t('两次输入的密码不一致')
    return
  }
  hint.value = ''
  let err: string | null
  if (vaultState.enabled) {
    if (!oldPw.value) {
      hint.value = t('请输入当前密码')
      return
    }
    err = await doChangePassword(oldPw.value, np)
  } else {
    err = await doSetPassword(np)
  }
  if (err) {
    hint.value = err
    // 错误同时弹 toast 更显眼, 避免"以为设置成功"(此前的静默失败隐患)
    showToast(err, t('操作失败'), 5000, 'error')
    return
  }
  close()
  // 设置/修改密码可能伴随数据库打开(锁定时进入管理), 广播刷新会话列表
  window.dispatchEvent(new CustomEvent('ldsshmanager:refresh-sessions'))
  showToast(
    vaultState.enabled ? t('锁屏密码已修改') : t('锁屏密码已设置'),
    t('提示'),
    5000,
    'success'
  )
}

async function disable() {
  if (!oldPw.value) {
    hint.value = t('请输入当前密码')
    return
  }
  if (!(await showConfirm(t('确认取消锁屏密码?'), t('取消锁屏密码'), t('确定'), t('取消')))) return
  const err = await doDisablePassword(oldPw.value)
  if (err) {
    hint.value = err
    return
  }
  close()
  window.dispatchEvent(new CustomEvent('ldsshmanager:refresh-sessions'))
  showToast(t('锁屏密码已取消'), t('提示'), 5000, 'success')
}
</script>

<template>
  <div v-if="vaultModal.visible" class="modal-overlay visible" @click.self="close">
    <div class="modal-dialog">
      <div class="modal-header">
        <h3 class="modal-title">{{ t('锁屏密码设置') }}</h3>
        <button class="modal-close" @click="close"><IconPark name="close" :size="14" :gap="0" /></button>
      </div>
      <div class="modal-body">
        <p class="dialog-desc">{{ t('锁屏密码用于加密本地数据, 请务必牢记!') }}</p>
        <div v-if="vaultState.enabled" class="form-row">
          <label class="form-label">{{ t('当前密码') }}</label>
          <input v-model="oldPw" type="text" class="form-input mask-pw" :placeholder="t('修改/取消时填写')" />
        </div>
        <div class="form-row">
          <label class="form-label">{{ t('新密码') }}</label>
          <input v-model="newPw" type="text" class="form-input mask-pw" :placeholder="t('至少 6 位')" />
        </div>
        <div class="form-row">
          <label class="form-label">{{ t('确认密码') }}</label>
          <input v-model="confirmPw" type="text" class="form-input mask-pw" :placeholder="t('再次输入新密码')" />
        </div>
        <p class="dialog-hint">{{ hint }}</p>
      </div>
      <div class="modal-actions">
        <button v-if="vaultState.enabled" class="btn btn-danger" @click="disable">{{ t('取消锁屏密码') }}</button>
        <button class="btn btn-default" @click="close">{{ t('关闭') }}</button>
        <button class="btn btn-primary" @click="save">
          {{ vaultState.enabled ? t('修改锁屏密码') : t('设置锁屏密码') }}
        </button>
      </div>
    </div>
  </div>
</template>
