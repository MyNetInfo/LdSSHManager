import {ref} from 'vue'
import {t} from './i18n'

// 主题管理: document.documentElement.dataset.theme = 'light' | 'dark'
// 持久化到 localStorage(对齐 LdTools 的行为)。

export type ThemeName = 'light' | 'dark'

const THEME_KEY = 'ldsshmanager-theme'

export const currentTheme = ref<ThemeName>(readStoredTheme())

function readStoredTheme(): ThemeName {
  try {
    const v = localStorage.getItem(THEME_KEY)
    if (v === 'light' || v === 'dark') return v
  } catch (e) {
    /* ignore */
  }
  return 'dark'
}

export function getTheme(): ThemeName {
  return currentTheme.value
}

export function setTheme(th: ThemeName) {
  currentTheme.value = th
  try {
    localStorage.setItem(THEME_KEY, th)
  } catch (e) {
    /* ignore */
  }
  applyThemeAttr()
}

export function toggleTheme(): ThemeName {
  const next: ThemeName = currentTheme.value === 'light' ? 'dark' : 'light'
  setTheme(next)
  return next
}

export function applyThemeAttr() {
  document.documentElement.dataset.theme = currentTheme.value
}

export function initTheme() {
  applyThemeAttr()
}

// 语言菜单等按钮需要随主题显示"当前主题"文字:
// light → "浅色"; dark → "深色" (图标 ☀/🌙 由模板按 currentTheme 渲染 IconPark)
export function themeLabel(th: ThemeName): string {
  return th === 'light' ? t('浅色') : t('深色')
}
