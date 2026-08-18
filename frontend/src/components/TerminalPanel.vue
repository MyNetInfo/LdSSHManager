<script lang="ts" setup>
import {onMounted, onUnmounted, ref, watch} from 'vue'
import {Terminal} from '@xterm/xterm'
import {FitAddon} from '@xterm/addon-fit'
import {WebLinksAddon} from '@xterm/addon-web-links'
import {WebglAddon} from '@xterm/addon-webgl'
import '@xterm/xterm/css/xterm.css'
import {EventsOn} from '../../wailsjs/runtime'
import {SSHSendData, SSHSessionResize, GetSetting} from '../../wailsjs/go/main/App'
import {t} from '../i18n'
import {currentTheme} from '../theme'
import type {SessionStatus} from '../types'
import {getTerminalTheme} from '../terminalThemes'

const props = defineProps<{
  sessionId: string
  focused: boolean
  // 连接状态: connecting(标签已开,SSH 连接中) → active(终端渲染) / failed(错误)
  status: SessionStatus
  error?: string
  host?: string
}>()

const emit = defineEmits<{
  (e: 'input', data: string): void
}>()

const containerRef = ref<HTMLDivElement | null>(null)
let term: Terminal | null = null
let fit: FitAddon | null = null
let resizeObserver: ResizeObserver | null = null
let offData: (() => void) | null = null
let offClose: (() => void) | null = null
let offSelection: (() => void) | null = null
let contextMenuCleanup: (() => void) | null = null
// ⚠️ 调试: 显示容器尺寸/cols/rows/字符宽, 定位 PTY 尺寸偏差根因(定位后移除)
const debugShow = ref(true)
const debugInfo = ref('')
// 最近 sendResize 同步成功的 cols (0=未成功过), 用于诊断 PTY 是否真的被同步
let lastSyncCols = 0
// 终端底部留空行数 (从系统设置读取, sendResize 时从 term.rows 扣除)
let bottomBlankRows = 0

async function loadBottomBlankRows() {
  try {
    const v = await GetSetting('terminal.bottomBlankRows')
    bottomBlankRows = v ? Math.max(0, Math.min(20, parseInt(v, 10) || 0)) : 0
  } catch { /* 保持默认 0 */ }
}

onMounted(() => {
  if (!containerRef.value) return
  loadBottomBlankRows()
  // 监听系统设置变更 (Topbar 保存设置后 dispatch)
  window.addEventListener('ldsshmanager:settings-changed', async (e: any) => {
    const key = e?.detail?.key
    if (key === 'terminal.bottomBlankRows') loadBottomBlankRows()
    if (key === 'terminal.theme' || !key) { await loadTerminalThemeSetting(); applyTerminalTheme() }
  })
  // connecting: 等 SSH 连接成功后 v-for key 变化重建本组件 → onMounted 重新执行 → initTerminal
  // (PTY 初始尺寸由后端默认 80x24, 对齐开源 Web SSH 标准, 不在连接时传前端测量值;
  // 连接后由 term.onResize 自动同步真实尺寸, 消除 measureAndNotify+兜底的 race condition)
  if (props.status !== 'active') return
  initTerminal()
})

// 共享终端配置: 正式终端与测量用同一套字体/字号/行高/字距
function terminalOptions() {
  return {
    cursorBlink: true,
    // 字号 13: 看着清晰(不"小"), Consolas 13px 字符宽 ~7.8px
    // → 1981px 容器经 accurateFit 修正后 cols ≈ 254(覆盖 top 全部列所需 ~130 列宽绰)
    fontSize: 13,
    fontFamily: 'Consolas, "Cascadia Code", "Courier New", monospace',
    fontWeight: 400,
    fontWeightBold: 600,
    letterSpacing: 0,
    lineHeight: 1.15,
    allowProposedApi: true,
  }
}

// 当前终端主题预设值 (从设置读取; 默认浅色)
let currentTerminalThemeValue = 'light'

async function loadTerminalThemeSetting() {
  try {
    const v = await GetSetting('terminal.theme')
    if (v) currentTerminalThemeValue = v
  } catch { /* 默认 dark */ }
}

function applyTerminalTheme() {
  if (!term) return
  term.options.theme = getTerminalTheme(currentTerminalThemeValue)
}

// active: 初始化正式终端
async function initTerminal() {
  await loadTerminalThemeSetting()
  const el = containerRef.value
  if (!el) return
  term = new Terminal({
    ...terminalOptions(),
    theme: getTerminalTheme(currentTerminalThemeValue),
    convertEol: false, // PTY 输出已含 CRLF，转换会多出 \r 导致光标错位（Tab 补全回显异常）
    rightClickSelectsWord: false,
  })

  fit = new FitAddon()
  term.loadAddon(fit)
  term.loadAddon(new WebLinksAddon())
  term.open(el)
  // WebGL 渲染器: 大幅提升 ANSI TUI(top/htop/vim 等)的高频重绘性能与清晰度.
  // 之前"WebGL 下 cols 偏差"根因是 xterm 内部 measureText 偏差, 已由 accurateFit
  // (浏览器原生 measureText 校准 dims.width) 修复, 恢复 WebGL 不再受影响.
  try {
    const webgl = new WebglAddon()
    webgl.onContextLoss(() => webgl.dispose())
    term.loadAddon(webgl)
  } catch (e) {
    console.warn('[terminal] WebGL renderer unavailable, fallback to default:', e)
  }
  accurateFit(term, fit, el)
  // 对齐开源 Web SSH 标准: xterm 自身 resize(fit/窗口变化) 时自动同步 PTY.
  // fit() 改变 cols/rows 会触发本回调 → SSHSessionResize → 远程 shell 收到 SIGWINCH,
  // top/htop/vim 等随后启动时读取的就是正确 PTY 尺寸.
  term.onResize(() => sendResize())
  updateDebug()

  // SSH 断开标记: 收到 ssh:close 后置 true, 冻结输入(按回车不再重发/不再刷 send error)
  let closed = false
  // send error 节流: 同 1s 内不重复刷屏
  let lastErrTs = 0

  term.onData((data) => {
    if (closed) return
    SSHSendData(props.sessionId, data).catch((err) => {
      const now = Date.now()
      if (now - lastErrTs > 1000) {
        lastErrTs = now
        term?.writeln(`\r\n[send error: ${err}]`)
      }
    })
    feedInput(data)
  })

  const selectionDisposer = term.onSelectionChange(() => {
    const sel = term?.getSelection()
    if (!sel) return
    navigator.clipboard.writeText(sel).catch(() => {})
  })
  offSelection = () => selectionDisposer.dispose()

  const onContextMenu = async (e: MouseEvent) => {
    e.preventDefault()
    try {
      const text = await navigator.clipboard.readText()
      if (text && term) {
        await SSHSendData(props.sessionId, text)
      }
    } catch {}
  }
  el.addEventListener('contextmenu', onContextMenu)
  contextMenuCleanup = () => {
    el.removeEventListener('contextmenu', onContextMenu)
  }

  resizeObserver = new ResizeObserver(() => {
    // fit → 尺寸变化 → term.onResize → sendResize 自动同步 PTY
    if (term && fit) accurateFit(term, fit, el)
    updateDebug()
  })
  resizeObserver.observe(el)

  // 连接建立时用后端默认尺寸, 这里 fit 后立即触发 onResize 同步一次 PTY 真实尺寸
  // (若 fit 后 cols/rows 与初始 80x24 不同, onResize 会自动发; 相同则跳过)
  sendResize(10) // 重试 10 次 (session 可能尚未在后端注册完成, "session not found" 时延迟重试)

  // ⚠️ 关键: SSH session 首次 data 到达说明 session 真正可用, 此时再同步 PTY 大小,
  // 覆盖初始 ConnectSSH 时 measureAndNotify 可能因容器 layout 未完成而算的偏差 cols.
  let firstDataSeen = false
  const dataHandler = (payload: {id: string; data: string}) => {
    if (payload.id !== props.sessionId || !term) return
    // base64 → Uint8Array：按原始字节写入，避免 string 边界处理导致的 ANSI 序列解析问题
    const bytes = Uint8Array.from(atob(payload.data), (c) => c.charCodeAt(0))
    term.write(bytes)
    // cd 联动: 检测到用户执行 cd 后发送的 pwd 输出, 解析实际目录并广播给文件管理器
    tryParsePendingCwd(payload.data)
    if (!firstDataSeen) {
      firstDataSeen = true
      // SSH session 真正 ready, 此刻强制同步 PTY cols/rows 到 xterm
      sendResize()
    }
  }

  const closeHandler = (payload: {id: string; reason: string}) => {
    if (payload.id !== props.sessionId || !term) return
    closed = true
    term.writeln(`\r\n[connection closed]`)
  }

  offData = EventsOn('ssh:data', dataHandler)
  offClose = EventsOn('ssh:close', closeHandler)

  if (props.focused) {
    term.focus()
  }
}

// ===========================================================================
// cd 联动: 检测用户执行 cd 命令 → 发送 pwd 查询实际目录 → 广播给文件管理器
// ===========================================================================
// 当前命令行输入缓冲(不含回车); 只在 active 终端 onData 时维护
let inputBuf = ''
// 是否在等待 pwd 输出(刚发送 pwd, 等 ssh:data 里的路径行)
let pendingCwd = false
// 已等待的数据块数, 超过上限放弃(避免一直挂着)
let pendingBlocks = 0
// 纯 cd 命令: `cd` 或 `cd <参数>`; 参数含 && ; | 等复合命令时不联动(避免误判)
const CD_RE = /^\s*cd(?:\s+(.+?))?\s*$/

// feedInput 维护命令行缓冲: 回车判定命令, 退格/Ctrl+U/Ctrl+C 修正缓冲, 转义序列丢弃
function feedInput(data: string) {
  for (const ch of data) {
    if (ch === '\x1b') {
      break // 方向键/功能键等转义序列: 丢弃本段余下内容(不影响缓冲)
    }
    if (ch === '\r' || ch === '\n') {
      onCommandEnter(inputBuf)
      inputBuf = ''
      continue
    }
    if (ch === '\x7f' || ch === '\b') {
      inputBuf = inputBuf.slice(0, -1)
      continue
    }
    if (ch === '\x15' || ch === '\x03') {
      inputBuf = '' // Ctrl+U / Ctrl+C
      continue
    }
    if (ch >= ' ') inputBuf += ch
  }
}

// onCommandEnter 缓冲为一整条命令(已回车)时: 若是纯 cd 命令 → 发 pwd 取实际目录
function onCommandEnter(cmd: string) {
  const m = CD_RE.exec(cmd)
  if (!m) return
  const arg = (m[1] || '').trim()
  // 复合命令(&& ; | 重定向等)不联动, 避免误判
  if (arg && /[&;|]/.test(arg.replace(/['"][^'"]*['"]/g, ''))) return
  pendingCwd = true
  pendingBlocks = 0
  SSHSendData(props.sessionId, 'pwd\r').catch(() => {
    pendingCwd = false
  })
}

// tryParsePendingCwd 解析 pwd 输出: strip ANSI 后取首个以 / 开头的行(即实际 cwd)
function tryParsePendingCwd(base64Data: string) {
  if (!pendingCwd) return
  pendingBlocks++
  try {
    const text = stripAnsi(atob(base64Data))
    const path = firstAbsPath(text)
    if (path) {
      pendingCwd = false
      window.dispatchEvent(
        new CustomEvent('ldsshmanager:sftp-cd', {
          detail: {sessionId: props.sessionId, path},
        })
      )
      return
    }
  } catch {}
  // 数据块可能拆包(回显/路径/提示符分几次到达), 最多等 3 块
  if (pendingBlocks > 3) pendingCwd = false
}

// stripAnsi 移除 CSI / OSC / 单字符转义序列, 保留可见文本
function stripAnsi(s: string): string {
  return s
    .replace(/\x1b\[[0-9;?]*[A-Za-z]/g, '')
    .replace(/\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)/g, '')
    .replace(/\x1b[()][0-9A-Za-z]/g, '')
}

// firstAbsPath 取文本中首个以 / 开头的行(pwd 输出; 回显的 "pwd" 和提示符不以 / 开头)
function firstAbsPath(text: string): string {
  for (const raw of text.split(/\r?\n/)) {
    const line = raw.trim()
    if (line.startsWith('/')) return line
  }
  return ''
}

// fit 后把 xterm 实际行列同步到后端 PTY.
// 重试机制: SSH session 在后端的注册可能晚于前端 initTerminal 调用, 第一次 "not found"
// 错误时延迟重试, 保证 PTY 真的同步到准确 cols (否则 top/htop/vim 等在窄 PTY 下启动缺列).
let resizePending = false
function sendResize(retry = 0) {
  if (!term) return
  if (resizePending) return
  resizePending = true
  SSHSessionResize(props.sessionId, term.cols, Math.max(1, term.rows - bottomBlankRows))
    .then(() => {
      resizePending = false
      if (term) lastSyncCols = term.cols // 记录同步成功, 便于诊断 PTY 是否真同步
      updateDebug()
    })
    .catch((err) => {
      resizePending = false
      const msg = String(err)
      if (/not found|not connected/i.test(msg) && retry > 0) {
        setTimeout(() => sendResize(retry - 1), 150)
      }
    })
}

// accurateFit: 用浏览器原生 canvas measureText 测字符宽 (绕开 xterm 内部 measurement 偏差),
// 覆盖 xterm 内部 dims.width 让后续 fit 用真实 cellW 重算 cols.
// 实测 WebView2: xterm 内部 measureText 估 16.7px, 浏览器 measureText 估 7.1px (真实), 2.3 倍偏差.
// 偏差导致 fit 算 cols 偏少 → SSH PTY cols 偏少 → top/htop/vim 等全屏 TUI 启动后缺列.
function accurateFit(t: Terminal, f: FitAddon, el: HTMLElement) {
  const opts = terminalOptions()
  const cssCellW = measureCssCellW(opts.fontSize, opts.fontFamily)
  if (cssCellW <= 0 || !isFinite(cssCellW)) {
    f.fit()
    return
  }
  // 覆盖 xterm 内部 dims.width, 让 f.fit() 用真实 cellW 算 cols
  const dims = (t as any)._core?._renderService?.dimensions?.css?.cell
  if (dims) dims.width = cssCellW
  f.fit()
}

// 浏览器原生 measureText 测字符宽. ctx.font 字符串必须与 xterm 内部一致(相同 fontSize/fontFamily).
// 这是修复 WebView2 中 xterm 内部 measureText 偏差的关键: xterm 自己 measureText 返回 16.7px,
// 浏览器 measureText 在同样的 ctx.font 下返回 7.1px (真实).
function measureCssCellW(fontSize: number, fontFamily: string): number {
  const c = document.createElement('canvas')
  const ctx = c.getContext('2d')
  if (!ctx) return 0
  ctx.font = `${fontSize}px ${fontFamily}`
  return ctx.measureText('M').width
}

// ⚠️ 调试(临时): 右下角显示 容器尺寸/cols/rows/字符宽(fit 估值 + canvas 实际), 用于定位 PTY 尺寸偏差
function updateDebug() {
  const el = containerRef.value
  if (!el || !term) return
  // fit 估值 (内部)
  let fitCellW = ''
  try {
    const cell = (term as any)._core?._renderService?.dimensions?.css?.cell
    if (cell && cell.width > 0) fitCellW = cell.width.toFixed(1)
  } catch {}
  // canvas 实际渲染反推
  let canvasCellW = ''
  try {
    const textLayer = el.querySelector('canvas.xterm-text-layer') as HTMLCanvasElement | null
    if (textLayer && term.cols > 0) {
      const dpr = window.devicePixelRatio || 1
      const w = textLayer.width / dpr / term.cols
      if (w > 0 && isFinite(w)) canvasCellW = w.toFixed(1)
    }
  } catch {}
  debugInfo.value = `${el.clientWidth}x${el.clientHeight}px cols=${term.cols} rows=${term.rows} cellW=${canvasCellW || '?'}px${fitCellW ? `(fit=${fitCellW})` : ''} sync=${lastSyncCols}`
}

watch(
  () => props.focused,
  (v) => {
    if (v && term) term.focus()
  }
)

onUnmounted(() => {
  offData?.()
  offClose?.()
  offSelection?.()
  contextMenuCleanup?.()
  resizeObserver?.disconnect()
  term?.dispose()
})
</script>

<template>
  <div ref="containerRef" class="terminal-panel">
    <!-- 连接中: 标签已开, SSH 尚未建立 -->
    <div v-if="props.status === 'connecting'" class="term-status">
      <span class="term-spinner"></span>
      <span>{{ t('连接') }} {{ props.host }}...</span>
    </div>
    <!-- 连接失败: 显示错误, 可关闭标签 -->
    <div v-else-if="props.status === 'failed'" class="term-status term-status--error">
      <div class="term-error-title">{{ t('连接失败') }}</div>
      <div class="term-error-msg">{{ props.error }}</div>
    </div>
    <!-- status === 'active': containerRef 由 xterm 填充 -->
    <div v-if="debugShow && props.status === 'active'" class="term-debug" :title="t('调试信息')">{{ debugInfo }}</div>
  </div>
</template>

<style scoped>
.terminal-panel {
  position: relative; /* term-debug 绝对定位参照 */
  width: 100%;
  height: 100%;
  background: var(--bg);
  overflow: hidden;
  padding: 4px;
  box-sizing: border-box;
}
.term-status {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: var(--text-2);
  user-select: none;
}
.term-status--error {
  color: var(--danger);
}
.term-error-title {
  font-weight: 600;
}
.term-error-msg {
  color: var(--text-2);
  max-width: 80%;
  text-align: center;
  word-break: break-all;
}
.term-spinner {
  width: 22px;
  height: 22px;
  border: 3px solid var(--border-2);
  border-top-color: var(--primary);
  border-radius: 50%;
  animation: term-spin 0.8s linear infinite;
}
/* ⚠️ 调试角标(临时): 显示容器尺寸/cols/rows/字符宽, 定位 PTY 尺寸偏差后移除 */
.term-debug {
  position: absolute;
  right: 10px;
  bottom: 8px;
  z-index: 20;
  padding: 3px 8px;
  font-size: 11px;
  font-family: Consolas, monospace;
  color: rgba(255, 255, 255, 0.9);
  background: rgba(0, 0, 0, 0.6);
  border-radius: 4px;
  pointer-events: none;
  white-space: nowrap;
  user-select: none;
}
@keyframes term-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
