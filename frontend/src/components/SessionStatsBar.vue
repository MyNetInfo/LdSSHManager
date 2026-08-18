<script lang="ts" setup>
// 服务器实时状态条: 显示在 TerminalTabs 内底部(终端下方/QuickCommandBar 上方)。
// 数据按类别不同频率采集:
//   base(系统/磁盘/CPU核线程)  → session 变化时取 1 次, 之后不再
//   runtime(运行时间)            → 60s 同步 boottime, 1s 本地计算显示
//   cpu/mem/nic                  → 3s
//   proc/io/conn/load            → 5s
//   swap                         → 30s

import {ref, watch, onUnmounted} from 'vue'
import * as AppApi from '../../wailsjs/go/main/App'
import {t} from '../i18n'

type WailsStats = {
  host: string
  cpuPercent: number
  cpuCores: number
  cpuThreads: number
  memTotal: number
  memUsed: number
  memPercent: number
  diskTotal: number
  diskUsed: number
  diskPercent: number
  procCount: number
  loadAvg: string
  uptime: string
  netRx: number
  netTx: number
  connCount: number
  swapTotal: number
  swapUsed: number
  swapPercent: number
  ioWait: number
  kernel: string
  boottime: number
  fetchedAt: number
}
const SessionStats = (AppApi as any).SessionStats as (
  id: string,
) => Promise<WailsStats | null>

const props = defineProps<{sessionId: string}>()

const stats = ref<WailsStats | null>(null)
const err = ref('')
const boottime = ref<number | null>(null)
const uptimeDisplay = ref('')

// 每个类别采集的字段 (后端 SessionStats 返回全量, 前端按类别只更新对应字段)
const CAT_FIELDS: Record<string, (keyof WailsStats)[]> = {
  base:    ['kernel', 'diskTotal', 'diskUsed', 'diskPercent', 'cpuCores', 'cpuThreads'],
  runtime: ['boottime'],
  cpu:     ['cpuPercent', 'cpuCores', 'cpuThreads'],
  mem:     ['memTotal', 'memUsed', 'memPercent'],
  swap:    ['swapTotal', 'swapUsed', 'swapPercent'],
  proc:    ['procCount'],
  io:      ['ioWait'],
  conn:    ['connCount'],
  nic:     ['netRx', 'netTx'],
  load:    ['loadAvg'],
}

// 采集间隔 (ms); base 只一次, runtime 60s
const CAT_INTERVAL_MS: Record<string, number> = {
  runtime: 60000,
  cpu:     3000,
  mem:     3000,
  swap:    30000,
  proc:    5000,
  io:      5000,
  conn:    5000,
  nic:     3000,
  load:    5000,
}

const timers: Record<string, number | null> = {}

async function fetchAndUpdate(cat: string) {
  if (!props.sessionId) return
  try {
    const next = await SessionStats(props.sessionId)
    if (!next) return
    if (!stats.value) stats.value = {} as WailsStats
    for (const f of CAT_FIELDS[cat]) {
      ;(stats.value as any)[f] = (next as any)[f]
    }
    err.value = ''
  } catch (e: any) {
    // 保留上次数据, 仅从未成功过时显示错误
    if (!stats.value) err.value = String(e && (e.message || e))
  }
}

// 一次全量拉取所有类别的数据(阶段一: 首次成功前用)
async function fetchFull(): Promise<boolean> {
  if (!props.sessionId) return false
  try {
    const next = await SessionStats(props.sessionId)
    if (!next) return false
    if (!stats.value) stats.value = {} as WailsStats
    for (const fields of Object.values(CAT_FIELDS)) {
      for (const f of fields) {
        ;(stats.value as any)[f] = (next as any)[f]
      }
    }
    if (next.boottime) boottime.value = next.boottime
    err.value = ''
    return true
  } catch (e) {
    // 首次成功前不显示错误, 由 3 秒重试兜底
    return false
  }
}

const sleep = (ms: number) => new Promise<void>(r => setTimeout(r, ms))

// 启动: 阶段一 立即全量拉一次, 失败每 3 秒重试直到首次成功;
// 首次成功后进入阶段二, 按各自频率 timer 运行 (base 只有这一次, 之后不再更新)。
// gen 计数器: sessionId 变化时旧的重试循环立即退出, 避免覆盖新会话数据。
let gen = 0
async function start() {
  const myGen = ++gen
  stop()
  if (!props.sessionId) return
  stats.value = null
  err.value = ''
  boottime.value = null
  uptimeDisplay.value = ''
  // ---- 阶段一: 全量拉取 + 3s 重试 ----
  for (;;) {
    if (myGen !== gen || !props.sessionId) return
    if (await fetchFull()) break
    await sleep(3000)
  }
  // ---- 阶段二: 按各自频率 timer ----
  for (const cat of Object.keys(CAT_INTERVAL_MS)) {
    const ms = CAT_INTERVAL_MS[cat]
    timers[cat] = window.setInterval(() => fetchAndUpdate(cat), ms)
  }
  tickUptime()
  timers['__uptime'] = window.setInterval(tickUptime, 1000)
}

function stop() {
  for (const k of Object.keys(timers)) {
    if (timers[k] != null) {
      clearInterval(timers[k]!)
      timers[k] = null
    }
  }
}

// 本地每秒基于 boottime 计算 "X天X小时X分X秒"
function tickUptime() {
  if (boottime.value && boottime.value > 0) {
    const secs = Math.max(0, Math.floor(Date.now() / 1000 - boottime.value))
    uptimeDisplay.value = formatUptime(secs)
  } else if (stats.value?.uptime) {
    uptimeDisplay.value = stats.value.uptime
  } else {
    uptimeDisplay.value = ''
  }
}

function formatUptime(secs: number): string {
  const d = Math.floor(secs / 86400)
  const h = Math.floor((secs % 86400) / 3600)
  const m = Math.floor((secs % 3600) / 60)
  const s = secs % 60
  return `${d}${t('天')}${h}${t('小时')}${m}${t('分')}${s}${t('秒')}`
}

watch(
  () => props.sessionId,
  (id) => {
    stop()
    stats.value = null
    err.value = ''
    boottime.value = null
    uptimeDisplay.value = ''
    if (id) start()
  },
  {immediate: true},
)

onUnmounted(stop)

// fmtCompact 字节自适应紧凑显示(无空格): 512M / 6.6G / 153G / 2G
function fmtCompact(bytes: number): string {
  if (bytes <= 0) return '0'
  const units = ['B', 'K', 'M', 'G', 'T']
  let v = bytes
  let u = 0
  while (v >= 1024 && u < units.length - 1) {
    v /= 1024
    u++
  }
  const s = v >= 100 ? v.toFixed(0) : v >= 10 ? v.toFixed(1) : v.toFixed(2)
  return parseFloat(s) + units[u]
}
</script>

<template>
  <!-- 服务器信息栏: 永远显示, 高度由 min-height 锁死, 不会因采集状态切换而跳变
       (SecureCRT 同款: 字段始终占位, 未采集到的值显示 "采集中…") -->
  <div class="stats-bar">
    <template v-if="!props.sessionId">
      <span class="stats-empty">{{ t('未选择会话') }}</span>
    </template>
    <template v-else>
      <span class="stat"><span class="lbl">[{{ t('系统') }}]</span>{{ stats?.kernel || t('采集中…') }}</span>
      <span class="stat"><span class="lbl">[{{ t('磁盘') }}]</span>{{ stats && stats.diskTotal > 0 ? fmtCompact(stats.diskUsed) + '/' + fmtCompact(stats.diskTotal) : t('采集中…') }}</span>
      <span class="stat"><span class="lbl">[CPU]</span>{{ stats && stats.cpuCores > 0 ? stats.cpuCores + t('核') + '/' + stats.cpuThreads + t('线程') + ' ' + stats.cpuPercent.toFixed(1) + '%' : t('采集中…') }}</span>
      <span class="stat"><span class="lbl">[{{ t('内存') }}]</span>{{ stats && stats.memTotal > 0 ? fmtCompact(stats.memUsed) + '/' + fmtCompact(stats.memTotal) : t('采集中…') }}</span>
      <span class="stat"><span class="lbl">[Swap]</span>{{ stats && stats.swapTotal > 0 ? fmtCompact(stats.swapUsed) + '/' + fmtCompact(stats.swapTotal) + ' ' + stats.swapPercent.toFixed(1) + '%' : '--' }}</span>
      <span class="stat"><span class="lbl">[{{ t('进程') }}]</span>{{ stats && stats.procCount > 0 ? stats.procCount : t('采集中…') }}</span>
      <span class="stat"><span class="lbl">[IO]</span>{{ stats ? stats.ioWait.toFixed(1) + '%' : t('采集中…') }}</span>
      <span class="stat"><span class="lbl">[{{ t('连接') }}]</span>{{ stats ? stats.connCount : t('采集中…') }}</span>
      <span class="stat"><span class="lbl">[{{ t('网卡') }}]</span>{{ stats ? '↓' + fmtCompact(stats.netRx) + '/s ↑' + fmtCompact(stats.netTx) + '/s' : t('采集中…') }}</span>
      <span class="stat"><span class="lbl">[{{ t('运行') }}]</span>{{ uptimeDisplay || t('采集中…') }}</span>
      <span class="stat"><span class="lbl">[{{ t('负载') }}]</span>{{ stats?.loadAvg || t('采集中…') }}</span>
    </template>
  </div>
</template>

<style scoped>
.stats-bar {
  min-height: 36px;
  flex: 0 0 auto;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 18px;
  padding: 6px 14px;
  background: var(--panel-3, #1b2230);
  border-top: 1px solid var(--border-2, #2a3344);
  color: var(--text, #d0d8e4); /* 与终端前景色一致 */
  font-size: 13px;
  overflow: hidden;
  white-space: nowrap;
  user-select: none;
}
.stats-bar > .stat,
.stats-bar > .stats-empty,
.stats-bar > .stats-err {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.stat .lbl {
  color: var(--text-2, #aab4c4); /* 标签稍暗以与值区分 */
}
.stat .pct {
  color: var(--text-2, #aab4c4);
}
.stats-empty {
  color: var(--text-2, #aab4c4);
}
.stats-err {
  color: var(--danger, #e24b4a);
}
</style>
