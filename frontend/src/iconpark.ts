// 统一图标注册表: 界面所有图标统一从这里取 (iconpark 图标集, 黑白优先)
// 用法: 模板里用 <IconPark name="lock" :size="16" :gap="4" />
// 约定: 只暴露语义名(lock/upload...), 不直接引用文件名; 新增图标只需在此追加一行。
// 选型规则(2026-08-18 用户强制): 浅色界面优先黑白 ico, 无黑白版才用彩色; 深色界面由 ICON_TONE + CSS filter 处理。
import setting from './assets/iconpark/setting-config.ico'
import upload from './assets/iconpark/upload.ico'
import download from './assets/iconpark/download.ico'
import exportIcon from './assets/iconpark/export.ico'
import importIcon from './assets/iconpark/import-and-export.ico'
import lock from './assets/iconpark/lock.ico'
import sun from './assets/iconpark/sun.ico'
import moon from './assets/iconpark/moon.ico'
import globe from './assets/iconpark/globe.ico'
import web from './assets/iconpark/web-page.ico'
import correct from './assets/iconpark/check.ico'
import user from './assets/iconpark/user.ico'
import folder from './assets/iconpark/folder.ico'
import folderOpen from './assets/iconpark/folder-open.ico'
import file from './assets/iconpark/file-doc.ico'
import edit from './assets/iconpark/edit-one.ico'
import deleteIcon from './assets/iconpark/delete.ico'
import add from './assets/iconpark/add-three.ico'
import refresh from './assets/iconpark/refresh.ico'
import up from './assets/iconpark/up.ico'
import close from './assets/iconpark/close.ico'
import right from './assets/iconpark/right.ico'
import link from './assets/iconpark/link.ico'
import downSquare from './assets/iconpark/down.ico'
import plus from './assets/iconpark/plus.ico'
import minus from './assets/iconpark/minus.ico'

export const ICONS = {
  setting,
  upload,
  download,
  export: exportIcon,
  import: importIcon,
  lock,
  sun,
  moon,
  globe,
  web,
  correct,
  user,
  folder,
  'folder-open': folderOpen,
  file,
  edit,
  delete: deleteIcon,
  add,
  refresh,
  up,
  close,
  right,
  link,
  'down-square': downSquare,
  plus,
  minus,
} as const

export type IconName = keyof typeof ICONS

// 深色主题下图标的处理方式(按图标是黑白还是彩色分档):
//   mono  → 黑白线条(黑色), 深色下看不见 → filter: invert(1) 反白
//   dark  → 彩色但整体偏暗, 深色下不醒目 → brightness(1.8) 提亮
//   color → 本身够亮, 深色下轻微提亮即可
export type IconTone = 'mono' | 'dark' | 'color'

export const ICON_TONE: Record<IconName, IconTone> = {
  setting: 'mono',
  upload: 'mono',
  download: 'mono',
  export: 'mono',
  import: 'mono',
  lock: 'dark',
  sun: 'mono',
  moon: 'dark',
  globe: 'dark',
  web: 'dark',
  correct: 'mono',
  user: 'dark',
  folder: 'color',
  'folder-open': 'dark',
  file: 'mono',
  edit: 'mono',
  delete: 'dark',
  add: 'mono',
  refresh: 'mono',
  up: 'mono',
  close: 'mono',
  right: 'mono',
  link: 'mono',
  'down-square': 'mono',
  plus: 'mono',
  minus: 'mono',
}
