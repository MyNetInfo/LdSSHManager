<template>
  <img
    :src="ICONS[name]"
    :style="{width: size + 'px', height: size + 'px', marginRight: gap + 'px', filter: iconFilter}"
    class="ip-icon"
    alt=""
    draggable="false"
  />
</template>

<script setup lang="ts">
import {computed} from 'vue'
import {ICONS, ICON_TONE, type IconName} from '../iconpark'
import {currentTheme} from '../theme'

// 统一图标渲染组件: 所有界面图标统一走这里, 样式一处控制。
// 深色主题下自动适配: 纯黑图标反白(mono)、偏暗彩色提亮(dark)、亮色轻微提亮(color)。
// name: 图标语义名(见 iconpark.ts); size: 显示尺寸(px); gap: 与右侧文字的间距(px)。
const props = withDefaults(
  defineProps<{
    name: IconName
    size?: number
    gap?: number
  }>(),
  {size: 16, gap: 4},
)

const isDark = computed(() => currentTheme.value === 'dark')

const iconFilter = computed(() => {
  if (!isDark.value) return undefined
  const tone = ICON_TONE[props.name]
  if (tone === 'mono') return 'invert(1)'
  if (tone === 'dark') return 'brightness(1.8) saturate(1.15)'
  return 'brightness(1.2)'
})
</script>

<style scoped>
.ip-icon {
  vertical-align: middle;
  flex-shrink: 0;
  user-select: none;
  pointer-events: none;
}
</style>
