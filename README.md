<!-- 语言切换 -->
[English](README.en.md) | **简体中文**

<div align="center">

# LdSSHManager

**一款桌面端 SSH 会话管理工具——会话树、多标签终端、快捷命令、SFTP、云端同步、Vault 加密，一站式多主机运维与远程开发**

[![Platform](https://img.shields.io/badge/平台-Windows%2010%2F11%20x64-blue)](#)
[![Version](https://img.shields.io/badge/版本-v0.8.46-green)](#)
[![Wails](https://img.shields.io/badge/Wails-v2-9cf)](#)
[![Vue](https://img.shields.io/badge/Vue-3-42b883)](#)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8)](#)
[![License](https://img.shields.io/badge/License-TBD-lightgrey)](LICENSE)

[官网](https://www.gxlidang.com/soft/ldsshmanager) · [下载安装](#-下载安装) · [构建开发](#-构建开发) · [问题反馈](#-问题反馈)

</div>

---

## 📖 项目简介

**LdSSHManager** 是一款基于 **Wails v2 + Go + Vue 3** 打造的桌面 SSH 会话管理工具，专为多主机运维、远程开发、服务器调试等场景设计。它把日常 SSH 操作中频繁用到的能力——**会话分组与连接、xterm 终端、常用命令快捷执行、远端文件浏览与编辑、多端配置同步、数据加密保护**——集中到一个轻量、原生体验的 Windows 桌面应用里，免去在不同终端/PuTTY/WinSCP/记事本之间来回切换的麻烦。

应用主打**本地优先 + 加密优先**：所有敏感数据（AES-256）、图片/附件、连接凭据都加密保存在本地；云端同步可选并支持端到端加密；同时集成快捷命令库、服务器状态实时面板、多语言与多主题，适合个人开发者与运维团队日常使用。

- 官网：<https://www.gxlidang.com/soft/ldsshmanager>
- 当前版本：**v0.8.46**

---

## ✨ 核心特性

### 会话与连接
- 🌳 **会话树分组管理**：外网 / 内网 / 自定义分组，双击一键连接，支持导入导出。
- ⚡ **两步连接**：双击立即打开标签显示 `connecting` 状态，后台异步建立 SSH 反馈 `connected` / `failed`，不等连接就能继续操作。
- 🔁 **cd 联动 SFTP**：在终端执行 `cd` 时自动更新文件管理器当前目录，文件浏览与终端无缝衔接。
- 🌐 **代理连接**：支持代理连接 SSH，适配受限网络环境。

### 终端体验
- 🖥️ **xterm 终端 + WebGL 渲染**：高频重绘（TUI / top / vim）更清晰流畅；真实 PTY 尺寸自适应窗口变化。
- 📑 **多标签并行**：多个服务器可同时连接、独立操作、随时关闭。
- ⌨️ **快捷命令库**：分类管理常用命令片段，一键执行，可附带回车自动确认。
- 🔄 **断线保护**：连接断开后冻结输入，停止 `send error` 刷屏，并显示 `[connection closed]` 提示。

### 文件与同步
- 📁 **SFTP 文件管理**：内置文件管理器，浏览、上传/下载、重命名、删除。
- ☁️ **云端同步**：配置 / 会话 / 快捷命令可选加密上传到云端，多端共享、随时迁移；支持版本回溯与冲突处理。
- 💾 **本地加密存储**：所有数据 SQLCipher (AES-256) 加密，密钥由锁屏密码派生；二进制附件存 `data/` 目录。

### 安全与体验
- 🔒 **锁屏保护**：闲置自动锁屏（可设置 0=关闭），防止他人窥探。
- 🗝️ **导出 / 导入**：全量数据可加密导出 / 导入，便于备份与迁移。
- 📊 **服务器状态实时面板**：底部状态栏显示 CPU / 内存 / 磁盘 / 进程 / 网络 / 运行时间等（类似 SecureCRT）。
- 🌗 **多主题**：浅色 / 深色主题自由切换。
- 🌍 **多语言界面**：简中 / 繁中 / English / 한국어 / 日本語 / Русский / Deutsch / Français。
- 🔌 **环境自检 + 一键安装**（开发者）：`INIT.bat` 自动检测/安装 Go、Node、MSYS2-GCC、NSIS、Wails CLI（开发者环境用），`MAKE.bat` 普通权限即可构建。

---

## 📸 界面预览

### 🖥️ 主界面 · 会话树 + 终端 + 文件管理器
左侧会话树按"外网 / 内网"分组，主机一键连接；中间多标签 xterm 终端实时执行命令；右侧 SFTP 文件管理器浏览 `/root` 目录；底部状态栏实时显示远程主机的系统/磁盘/CPU/内存/进程/IO/连接/网卡/运行/负载等指标（类似 SecureCRT 信息栏）。
![Main](images/11.jpg)

### ⌨️ 快捷命令栏
左侧"快捷命令"面板按分类列出常用命令（如 `基础配置 / 密码 / 密钥 / Easytier / MacOS / AI / 常用命令 / 系统检查 / 防火墙 / 端口占用 / ...`），双击或在终端中按快捷键即可执行。
![QuickCommand](images/22.jpg)

### 📤 数据导出（锁屏密码加密）
"导出数据"对话框提示：导出数据**用锁屏密码加密**，忘记密码将无法恢复，请务必牢记。
![Export](images/33.jpg)

### ☁️ 云端版本选择
云端下载 / 同步时，可选择历史版本（`版本 1 ~ 版本 7 (最新)`），带时间戳，方便回溯与冲突处理。
![CloudVersions](images/44.jpg)

### 🔐 输入云端加密密码
云端同步 / 上传需输入加密密码（至少 6 位），用于端到端加密保护你的连接配置。
![CloudPassword](images/55.jpg)

### 🌐 多语言切换
顶部 **Language** 下拉菜单支持 7 种语言一键切换：English / 繁體中文 / 한국어 / 日本語 / Русский / Deutsch / Français（始终显示英文 "Language" 标签，方便识别）。
![Language](images/66.jpg)

### ⚙️ 系统设置
简洁的"系统设置"弹窗，可配置**终端风格**（浅色 / 深色 / 自定义）、**终端底部留空行数**、**自动锁屏**时长（分钟，0=不自动锁）。
![Settings](images/77.jpg)

---

## 🏗️ 技术栈

| 层 | 选型 | 说明 |
|---|---|---|
| 桌面框架 | **Wails v2** | Go + Web 跨平台桌面，原生窗口 + WebView2 |
| 后端 | **Go 1.25+** | Wails 绑定、SSH 客户端（CGO 依赖 go-sqlcipher）、SQLCipher 加密、并发 pump |
| 前端 | **Vue 3 (TypeScript) + Vite** | 组件化、TypeScript 类型安全 |
| 终端 | **xterm.js + WebGL 渲染** | 高性能 TUI 渲染 |
| 数据库 | **SQLite + SQLCipher (AES-256)** | 加密本地数据库；密钥由锁屏密码派生 |
| 通信 | Wails bindings + Go channels | 前后端事件 / 调用 |

---

## 🔒 安全与隐私

| 项 | 机制 |
|---|---|
| 本地数据库 | **SQLCipher (AES-256)** 全库加密，密钥由用户锁屏密码经 PBKDF2 派生；不输入正确密码无法解密 |
| 二进制附件 | 存放到 exe 所在目录下的 `data/` 目录，不上传（除非显式云端同步） |
| 锁屏保护 | 应用闲置自动锁屏（可在系统设置中配置时长，0=不自动锁） |
| 云端同步 | 端到端加密：用户设置的"加密密码"（≥6 位）独立于锁屏密码，忘记则云端数据无法恢复 |
| 数据导出 | 同云端加密规则，用锁屏密码加密 |
| 内存防御 | store / vault 等关键路径对 `nil` 数据库 / 未注入 rekey 做了显式防御，避免静默数据丢失 |

---

## 📦 下载安装

### 官方渠道
👉 **https://www.gxlidang.com/soft/ldsshmanager**

提供两个安装包：

| 包 | 文件 | 适用场景 |
|---|---|---|
| 🟢 **绿色便携版** | `LdSSHManager-x64.exe` | 免安装，双击即可使用，适合临时使用 / U 盘携带 |
| 📦 **NSIS 安装版** | `LdSSHManager-Install-x64.exe` | 标准 Windows 安装包，可自定义安装目录、桌面/开始菜单快捷方式、卸载 |

> 下载后如提示"未知发布者"，点击"更多信息"→"仍要运行"即可（未购买代码签名证书）。

### 系统要求

- **操作系统**：Windows 10 / 11（x64）
- **WebView2 Runtime**：Win11 已预装；Win10 首次启动会提示安装（**NSIS 安装版**已内嵌 WebView2，无需额外下载）
- **屏幕分辨率**：建议 ≥ 1280×720

---

## 🛠️ 构建开发

开发者如需自行构建，环境要求见 [AGENTS.md](./AGENTS.md)。

### 源码结构

```
LdSSHManager/
├── app.go               # Wails 绑定方法
├── internal/            # Go 后端业务（store / vault / sshclient）
├── frontend/            # Vue3 前端
│   └── src/components/  # 主要 UI 组件
├── MAKE.bat             # 构建脚本（普通权限）
├── INIT.bat             # 环境检测 + 安装脚本（首次/新机手动执行，需管理员）
├── DEV.bat              # wails dev 热开发
├── main.go              # Wails 入口
├── bump_version.mjs     # 提交前自动递增版本号
└── ___docs___/CHANGES.md # 重大改动记录
```

### 快速开始

```powershell
# 1. 首次/新机：手动执行一次环境安装（双击 INIT.bat 或在管理员 cmd 中执行，会弹一次 UAC）
INIT.bat

# 2. 构建（普通权限，不需要管理员）
MAKE.bat
```

`MAKE.bat` 会自动完成：svn 更新 → 拼 PATH → 环境检查 → `wails build -platform windows/amd64 -nsis -webview2 embed` → 把产物移动到项目根目录。

### 提交前必做

- 运行 `node bump_version.mjs` 自动递增 `app.go` 中的 `GetVersion()` 版本号。
- 提交 SVN，注释格式：`【{hostname}】LdSSHManager - {修改说明}`，务必带新版本号。

---

## 🌐 多语言

应用内建 7 种语言界面：

- 🇨🇳 简体中文（默认）
- 🇹🇼 繁體中文
- 🇺🇸 English
- 🇰🇷 한국어
- 🇯🇵 日本語
- 🇷🇺 Русский
- 🇩🇪 Deutsch
- 🇫🇷 Français

顶部 **Language** 按钮（始终显示英文）一键切换；语言包集中维护于 `frontend/src/i18n.ts`。

---

## 📁 目录结构要点

```
frontend/src/components/
├── SessionTree.vue     # 会话树 + 新建/编辑弹窗
├── QuickCommandBar.vue # 底部快捷命令栏（宽 920px 编辑弹窗）
├── TerminalPanel.vue   # xterm 终端 + WebGL 渲染 + PTY 同步
├── TerminalTabs.vue    # 标签栏
├── Topbar.vue          # 顶部菜单 + 系统设置 + 云同步
├── LockScreen.vue      # 锁屏
├── SessionStatsBar.vue # 底部服务器状态实时面板
├── FileManager.vue     # SFTP 文件管理
├── Dialog.vue + dialog.ts  # 统一 Toast/Confirm/Prompt
└── ...
```

---

## ❓ 常见问题

<details>
<summary><b>下载后 Windows 提示"未知发布者"怎么办？</b></summary>
属于正常现象：项目未购买商业代码签名证书。点击"更多信息" → "仍要运行" 即可。
</details>

<details>
<summary><b>Win10 启动时提示安装 WebView2？</b></summary>
下载 **NSIS 安装版**（`LdSSHManager-Install-x64.exe`）已内嵌 WebView2，安装包会自动完成；如使用便携版则需按提示下载安装（约 100MB，一次性）。
</details>

<details>
<summary><b>忘了锁屏密码怎么办？</b></summary>
**无法恢复**——数据库（SQLCipher）密钥由锁屏密码派生，密码丢了数据就解不开。建议：
1. 定期用"导出数据"功能做加密备份，导出文件可用同一锁屏密码导入恢复；
2. 在密码管理器中保存一份。
</details>

<details>
<summary><b>云端同步的"加密密码"忘了怎么办？</b></summary>
同上，**无法恢复**——加密密码独立于锁屏密码。建议在密码管理器中记录。
</details>

---

## 🔗 链接

- 🌐 **官网（含下载）**：<https://www.gxlidang.com/soft/ldsshmanager>
- 🐛 **问题反馈**：提交 GitHub Issue
- 💬 **建议交流**：官网评论区 / 邮件

---

## 🤝 致谢

- 终端渲染：[xterm.js](https://xtermjs.org/) & [xterm-webgl](https://github.com/xtermjs/xterm.js)
- 桌面框架：[Wails](https://wails.io/)
- 加密：[SQLCipher](https://www.zetetic.net/sqlcipher/)

---

## 📄 License

本仓库的开源协议详见 [`LICENSE`](LICENSE) 文件。

> ⚠️ 本项目代码以"现状"提供，使用前请自行评估安全风险并遵循当地法律法规。

---

<sub align="center">用 ❤️ 打造 · Made for SSH lovers</sub>
