// 字号缩放: 通过 document.documentElement.style.zoom 整体缩放 (0.8~1.6)。
// 持久化到 localStorage(对齐 LdTools 的行为)。

const FONT_MIN = 0.8
const FONT_MAX = 1.6
const FONT_KEY = 'ldsshmanager-font-scale'

export function readFontScale(): number {
  let scale = 1.0
  try {
    const v = parseFloat(localStorage.getItem(FONT_KEY) || '')
    if (v >= FONT_MIN && v <= FONT_MAX) scale = v
  } catch (e) {
    /* ignore */
  }
  return scale
}

export function applyFontScale() {
  const scale = readFontScale()
  ;(document.documentElement as HTMLElement).style.zoom = String(scale)
}

export function fontStep(delta: number): number {
  const scale = Math.min(FONT_MAX, Math.max(FONT_MIN, Math.round((readFontScale() + delta) * 10) / 10))
  try {
    localStorage.setItem(FONT_KEY, String(scale))
  } catch (e) {
    /* ignore */
  }
  ;(document.documentElement as HTMLElement).style.zoom = String(scale)
  return scale
}
