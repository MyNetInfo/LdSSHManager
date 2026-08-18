<template>
  <teleport to="body">
    <!-- Transient notice: auto-dismisses after 5s -->
    <transition name="dlg-toast">
      <div
        v-if="state.visible && state.kind === 'toast'"
        class="dlg-toast"
        :class="{'dlg-toast--error': state.variant === 'error'}"
      >
        <button class="dlg-toast-close" type="button" title="Dismiss" @click="cancel"><IconPark name="close" :size="12" :gap="0" /></button>
        <div v-if="state.title" class="dlg-toast-title">{{ state.title }}</div>
        <div class="dlg-toast-msg">{{ state.message }}</div>
        <div v-if="state.actions.length" class="dlg-toast-actions">
          <button
            v-for="a in state.actions"
            :key="a.value"
            class="dlg-toast-btn"
            type="button"
            @click="action(a.value)"
          >{{ a.label }}</button>
        </div>
      </div>
    </transition>

    <!-- Modal: prompt (input) or confirm (yes/no) -->
    <transition name="dlg-fade">
      <div
        v-if="state.visible && (state.kind === 'prompt' || state.kind === 'confirm')"
        class="dlg-overlay"
        @click.self="cancel"
      >
        <div class="dlg-modal">
          <h3 v-if="state.title" class="dlg-title">{{ state.title }}</h3>
          <p v-if="state.message" class="dlg-msg">{{ state.message }}</p>
          <input
            v-if="state.kind === 'prompt'"
            ref="inputEl"
            class="dlg-input"
            :type="state.masked ? 'password' : 'text'"
            v-model="state.inputValue"
            @keyup.enter="confirm"
          />
          <div class="dlg-actions">
            <button class="dlg-btn dlg-cancel" type="button" @click="cancel">
              {{ state.cancelText }}
            </button>
            <button class="dlg-btn dlg-ok" type="button" @click="confirm">
              {{ state.confirmText }}
            </button>
          </div>
        </div>
      </div>
    </transition>
  </teleport>
</template>

<script lang="ts" setup>
import {ref, watch, nextTick} from 'vue'
import {useDialog, dialogConfirm, dialogCancel, dialogAction} from '../dialog'
import IconPark from './IconPark.vue'

const state = useDialog()
const inputEl = ref<HTMLInputElement | null>(null)

function confirm() {
  dialogConfirm()
}
function cancel() {
  dialogCancel()
}
function action(value: string) {
  dialogAction(value)
}

// Auto-focus the input when a prompt opens so the user can type immediately.
watch(
  () => state.visible,
  (visible) => {
    if (visible && state.kind === 'prompt') {
      nextTick(() => inputEl.value?.focus())
    }
  }
)
</script>

<style scoped>
.dlg-toast {
  position: fixed;
  right: 20px;
  bottom: 20px;
  max-width: 340px;
  background: var(--panel-2);
  color: var(--text);
  border: 1px solid var(--border-2);
  border-left: 3px solid var(--primary);
  border-radius: 5px;
  padding: 12px 14px;
  padding-right: 30px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
  z-index: 1000;
}
.dlg-toast--error {
  border-left-color: var(--danger);
}
.dlg-toast-title {
  font-weight: 700;
  margin-bottom: 4px;
}
.dlg-toast-msg {
  color: var(--text-2);
  word-break: break-word;
}
.dlg-toast-close {
  position: absolute;
  top: 6px;
  right: 8px;
  width: 18px;
  height: 18px;
  line-height: 16px;
  text-align: center;
  border: none;
  background: transparent;
  color: var(--text-2);
  cursor: pointer;
  border-radius: 3px;
}
.dlg-toast-close:hover {
  background: rgba(255, 255, 255, 0.08);
  color: var(--input-text);
}
.dlg-toast-actions {
  display: flex;
  gap: 8px;
  margin-top: 10px;
}
.dlg-toast-btn {
  padding: 5px 12px;
  border: 1px solid var(--border-2);
  border-radius: 3px;
  background: var(--hover-2);
  color: var(--text);
  cursor: pointer;
}
.dlg-toast-btn:hover {
  background: var(--hover-2);
}
.dlg-toast-btn.dlg-toast-btn--primary {
  background: var(--primary);
  border-color: var(--primary);
  color: #fff;
}
.dlg-toast-btn.dlg-toast-btn--primary:hover {
  background: var(--primary-hover);
}
.dlg-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}
.dlg-modal {
  background: var(--panel-2);
  color: var(--text);
  padding: 18px;
  border-radius: 6px;
  min-width: 300px;
  max-width: 420px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
}
.dlg-title {
  margin: 0 0 12px;
}
.dlg-msg {
  margin: 0 0 12px;
  color: var(--text-2);
  word-break: break-word;
}
.dlg-input {
  width: 100%;
  box-sizing: border-box;
  padding: 6px 8px;
  border: 1px solid var(--border-2);
  border-radius: 3px;
  background: var(--panel);
  color: var(--input-text);
}
.dlg-input:focus {
  outline: none;
  border-color: var(--primary);
}
.dlg-actions {
  display: flex;
  gap: 10px;
  margin-top: 14px;
}
.dlg-btn {
  flex: 1;
  padding: 7px;
  border: 1px solid var(--border-2);
  border-radius: 3px;
  background: var(--hover-2);
  color: var(--text);
  cursor: pointer;
}
.dlg-btn.dlg-ok {
  background: var(--primary);
  border-color: var(--primary);
  color: #fff;
}
.dlg-btn.dlg-ok:hover {
  background: var(--primary-hover);
}
.dlg-btn.dlg-cancel:hover {
  background: var(--hover-2);
}
/* Backdrop fade for prompt/confirm modals */
.dlg-fade-enter-active,
.dlg-fade-leave-active {
  transition: opacity 0.18s ease;
}
.dlg-fade-enter-from,
.dlg-fade-leave-to {
  opacity: 0;
}

/* Modal pop-in (runs on mount of the inner card) */
@keyframes dlg-pop {
  from {
    opacity: 0;
    transform: translateY(-10px) scale(0.96);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}
.dlg-modal {
  animation: dlg-pop 0.2s ease;
}

/* Toast slides in from the right + fades; same on the way out */
.dlg-toast-enter-active,
.dlg-toast-leave-active {
  transition: opacity 0.22s ease, transform 0.22s ease;
}
.dlg-toast-enter-from,
.dlg-toast-leave-to {
  opacity: 0;
  transform: translateX(24px);
}
</style>
