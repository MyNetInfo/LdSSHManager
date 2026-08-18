export interface SavedSession {
  id: number
  name: string
  host: string
  port: number
  username: string
  authType: string
  keyPath: string
  groupName: string
  sortOrder: number
  proxyType: string
  proxyHost: string
  proxyPort: number
  proxyUsername: string
  execCmd: string
  password: string
}

export type SessionStatus = 'connecting' | 'active' | 'failed'

export interface ActiveSession {
  id: string
  name: string
  host: string
  sessionId: number
  // 连接状态: connecting(标签已开, SSH 连接中) → active(成功) / failed(失败)
  status: SessionStatus
  error?: string
}

export interface SFTPEntry {
  name: string
  path: string
  size: number
  mode: string
  modTime: number
  isDir: boolean
  owner: string
  group: string
}

export interface ConnectForm {
  password: string
  keyPEM: string
}

// 底部快捷命令栏的按钮: 点击向当前终端发送 command(可选附加回车)。
export interface QuickCommand {
  id: number
  name: string
  command: string
  withEnter: boolean
  sortOrder: number
  category: string
}
