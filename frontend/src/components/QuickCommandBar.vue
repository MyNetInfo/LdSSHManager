<script lang="ts" setup>
import {onMounted, onBeforeUnmount, ref, computed} from 'vue'
import {ListQuickCommands, SSHSendData} from '../../wailsjs/go/main/App'
import {showToast} from '../dialog'
import {t} from '../i18n'
import type {QuickCommand} from '../types'

const props = defineProps<{
  // 当前激活(已连接)会话的 id; 为空表示没有可用终端, 点击按钮时提示先连接。
  currentSessionId: string
}>()

const commands = ref<QuickCommand[]>([])
// 选中分类: 空字符串 = 全部
const selectedCategory = ref('')

// 所有分类(去重并按 localeCompare 排序), 空串代表"未分类"。
const categories = computed(() => {
  const set = new Set<string>()
  for (const c of commands.value) set.add(c.category || '')
  return [...set].sort((a, b) => a.localeCompare(b))
})

// 按选中分类过滤后的命令列表(按名称字母序排序, 与编辑面板一致)
const filtered = computed(() => {
  let list = !selectedCategory.value ? commands.value : commands.value.filter((c) => (c.category || '') === selectedCategory.value)
  return [...list].sort((a, b) => a.name.localeCompare(b.name))
})

async function load() {
  try {
    commands.value = (await ListQuickCommands()) || []
    // 选中的分类被删光后, 重置回"全部"
    if (selectedCategory.value && !categories.value.includes(selectedCategory.value)) {
      selectedCategory.value = ''
    }
  } catch (err: any) {
    console.error('load quick commands:', err)
  }
}

// 左则快捷命令增删改后同步刷新。
onMounted(() => {
  load()
  window.addEventListener('ldsshmanager:refresh-sessions', load)
  window.addEventListener('ldsshmanager:refresh-quick-commands', load)
})
onBeforeUnmount(() => {
  window.removeEventListener('ldsshmanager:refresh-sessions', load)
  window.removeEventListener('ldsshmanager:refresh-quick-commands', load)
})

// 点击按钮: 把命令发送到当前终端。withEnter 时附加回车(自动执行)。
async function send(cmd: QuickCommand) {
  if (!props.currentSessionId) {
    showToast(t('请先连接一个会话'), '', 5000, 'error')
    return
  }
  const text = cmd.withEnter ? cmd.command + '\r' : cmd.command
  try {
    await SSHSendData(props.currentSessionId, text)
  } catch (err: any) {
    showToast(String(err), '', 5000, 'error')
  }
}
</script>

<template>
  <section class="qc-bar">
    <div class="qc-filter">
      <select v-model="selectedCategory" class="qc-select" :title="t('按分类筛选')">
        <option value="">{{ t('全部') }}</option>
        <option v-for="cat in categories" :key="cat" :value="cat">{{ cat || t('未分类') }}</option>
      </select>
    </div>
    <div class="qc-list">
      <button
        v-for="cmd in filtered"
        :key="cmd.id"
        class="qc-btn"
        :title="cmd.command"
        @click="send(cmd)"
      >{{ cmd.name }}</button>
      <span v-if="filtered.length === 0" class="qc-empty">{{ t('没有快捷命令') }}</span>
    </div>
  </section>
</template>

<style scoped>
.qc-bar {
  display: flex;
  align-items: center;
  height: 38px;
  flex-shrink: 0;
  padding: 0 10px;
  background: var(--panel);
  border-top: 1px solid var(--border);
}
.qc-filter {
  flex-shrink: 0;
  margin-right: 10px;
}
.qc-select {
  height: 26px;
  padding: 0 6px;
  color: var(--text);
  background: var(--panel-2);
  border: 1px solid var(--border-2);
  border-radius: 5px;
  font-size: 13px;
  cursor: pointer;
  outline: none;
}
.qc-select:focus {
  border-color: var(--accent);
}
.qc-list {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  overflow-x: auto;
  overflow-y: hidden;
  height: 100%;
}
.qc-btn {
  flex-shrink: 0;
  padding: 4px 12px;
  color: var(--text);
  background: var(--panel-2);
  border: 1px solid var(--border-2);
  border-radius: 5px;
  cursor: pointer;
  white-space: nowrap;
  transition: background 0.15s, border-color 0.15s, color 0.15s;
  font-size: 13px;
}
.qc-btn:hover {
  background: var(--primary);
  border-color: var(--primary);
  color: #fff;
}
.qc-empty {
  color: var(--text-3);
  white-space: nowrap;
  font-size: 13px;
}
</style>
