import {reactive} from 'vue'
import {t} from './i18n'

// Unified dialog/toast service for the whole app.
// Replaces native window.prompt / window.confirm / window.alert so every
// interactive surface looks the same and follows the "5-second" rule.
//
// - toast: transient notice, auto-dismisses after `autoCloseMs` (default 5000).
//   Can carry optional action buttons (e.g. retry / dismiss) and a close (x).
// - confirm: yes/no question, stays until the user clicks.
// - prompt: text input, stays until the user submits or cancels.
//
// The visual is rendered by components/Dialog.vue, which reads this shared
// reactive state. Mount <Dialog /> once (in App.vue).

export type DialogKind = 'prompt' | 'confirm' | 'toast'
export type DialogVariant = 'info' | 'success' | 'error'

export interface ToastAction {
  label: string
  value: string
}

interface DialogState {
  visible: boolean
  kind: DialogKind
  variant: DialogVariant
  title: string
  message: string
  defaultValue: string
  inputValue: string
  confirmText: string
  cancelText: string
  resolve: ((value: any) => void) | null
  autoCloseMs: number
  actions: ToastAction[]
  masked: boolean
}

const state = reactive<DialogState>({
  visible: false,
  kind: 'toast',
  variant: 'info',
  title: '',
  message: '',
  defaultValue: '',
  inputValue: '',
  confirmText: t('确定'),
  cancelText: t('取消'),
  resolve: null,
  autoCloseMs: 5000,
  actions: [],
  masked: false,
})

let autoTimer: ReturnType<typeof setTimeout> | null = null

function clearTimer() {
  if (autoTimer !== null) {
    clearTimeout(autoTimer)
    autoTimer = null
  }
}

// Single exit path: resolves the pending promise (if any) and hides the dialog.
// Keeping one place that closes the dialog avoids the old per-branch mess and
// guarantees the 5-second timer is always cleared.
function hide(result: any) {
  clearTimer()
  if (state.resolve) {
    state.resolve(result)
    state.resolve = null
  }
  state.visible = false
}

export function useDialog() {
  return state
}

// Returns a Promise that resolves to the clicked action value, or null when the
// toast auto-closes or is dismissed. Callers that only need to show a notice can
// ignore the return value.
export function showToast(
  message: string,
  title = '',
  durationMs = 5000,
  variant: DialogVariant = 'info',
  actions: ToastAction[] = []
): Promise<string | null> {
  clearTimer()
  state.kind = 'toast'
  state.variant = variant
  state.title = title
  state.message = message
  state.actions = actions
  state.autoCloseMs = durationMs
  state.visible = true
  return new Promise<string | null>((resolve) => {
    state.resolve = resolve
    autoTimer = setTimeout(() => hide(null), durationMs)
  })
}

export function showConfirm(
  message: string,
  title = t('确认'),
  confirmText = t('确定'),
  cancelText = t('取消')
): Promise<boolean> {
  clearTimer()
  state.kind = 'confirm'
  state.variant = 'info'
  state.title = title
  state.message = message
  state.confirmText = confirmText
  state.cancelText = cancelText
  state.actions = []
  state.visible = true
  return new Promise<boolean>((resolve) => {
    state.resolve = resolve
  })
}

export function showPrompt(
  message: string,
  defaultValue = '',
  title = t('输入'),
  confirmText = t('确定'),
  cancelText = t('取消'),
  masked = false
): Promise<string | null> {
  clearTimer()
  state.kind = 'prompt'
  state.variant = 'info'
  state.title = title
  state.message = message
  state.defaultValue = defaultValue
  state.inputValue = defaultValue
  state.confirmText = confirmText
  state.cancelText = cancelText
  state.masked = masked
  state.actions = []
  state.visible = true
  return new Promise<string | null>((resolve) => {
    state.resolve = resolve
  })
}

export function dialogConfirm() {
  if (state.kind === 'prompt') {
    hide(state.inputValue)
  } else if (state.kind === 'confirm') {
    hide(true)
  } else {
    hide(null)
  }
}

export function dialogCancel() {
  if (state.kind === 'prompt') {
    hide(null)
  } else if (state.kind === 'confirm') {
    hide(false)
  } else {
    hide(null)
  }
}

// Fired by an action button inside a toast. Resolves the toast promise with the
// action's value so callers can react (e.g. retry a failed operation).
export function dialogAction(value: string) {
  hide(value)
}
