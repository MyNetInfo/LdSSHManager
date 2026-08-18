/**
 * 终端主题预设 (xterm.js ITheme)
 * 供 TerminalPanel 应用、Topbar 设置选择器使用。
 */
import type {ITheme} from '@xterm/xterm'

export interface TerminalThemePreset {
  name: string       // 显示名
  value: string      // 存储值
  theme: ITheme      // xterm ITheme 对象
}

export const TERMINAL_THEME_PRESETS: TerminalThemePreset[] = [
  {
    name: '浅色 (默认)',
    value: 'light',
    theme: {
      background: '#ffffff', foreground: '#1e2328', cursor: '#0066cc',
      selectionBackground: '#b3d7ff',
      black: '#5c5c5c', red: '#cd3131', green: '#00bc00', yellow: '#949800',
      blue: '#0451a5', magenta: '#bc05bc', cyan: '#0598bc', white: '#555555',
    },
  },
  {
    name: '深色',
    value: 'dark',
    theme: {
      background: '#0f141e', foreground: '#d0d8e4', cursor: '#4f8cff',
      selectionBackground: '#264f78',
      black: '#1b2636', red: '#ff7b7b', green: '#56d4a0', yellow: '#f7c95c',
      blue: '#4f8cff', magenta: '#c084fc', cyan: '#5ce1e6', white: '#e8eef5',
    },
  },
  {
    name: 'Solarized Dark',
    value: 'solarized-dark',
    theme: {
      background: '#002b36', foreground: '#839496', cursor: '#839496',
      selectionBackground: '#073642',
      black: '#073642', red: '#dc322f', green: '#859900', yellow: '#b58900',
      blue: '#268bd2', magenta: '#d33682', cyan: '#2aa198', white: '#eee8d5',
    },
  },
  {
    name: 'Solarized Light',
    value: 'solarized-light',
    theme: {
      background: '#fdf6e3', foreground: '#657b83', cursor: '#657b83',
      selectionBackground: '#eee8d5',
      black: '#073642', red: '#dc322f', green: '#859900', yellow: '#b58900',
      blue: '#268bd2', magenta: '#d33682', cyan: '#2aa198', white: '#002b36',
    },
  },
  {
    name: 'Monokai',
    value: 'monokai',
    theme: {
      background: '#272822', foreground: '#f8f8f2', cursor: '#f8f8f2',
      selectionBackground: '#49483e',
      black: '#272822', red: '#f92672', green: '#a6e22e', yellow: '#f4bf75',
      blue: '#66d9ef', magenta: '#ae81ff', cyan: '#a1efe4', white: '#f8f8f2',
    },
  },
  {
    name: 'Dracula',
    value: 'dracula',
    theme: {
      background: '#282a36', foreground: '#f8f8f2', cursor: '#f8f8f2',
      selectionBackground: '#44475a',
      black: '#21222c', red: '#ff5555', green: '#50fa7b', yellow: '#f1fa8c',
      blue: '#bd93f9', magenta: '#ff79c6', cyan: '#8be9fd', white: '#f8f8f2',
    },
  },
]

/** 根据预设值获取 xterm ITheme，找不到则返回第一个预设(浅色默认) */
export function getTerminalTheme(presetValue: string): ITheme {
  const found = TERMINAL_THEME_PRESETS.find(p => p.value === presetValue)
  return found ? found.theme : TERMINAL_THEME_PRESETS[0].theme
}
