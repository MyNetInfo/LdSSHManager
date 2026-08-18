<!-- 语言切换 -->
[English](README.en.md) | **简体中文**

<div align="center">

# LdSSHManager

**开源 · 可定制 · 数据加密有保障 —— 一站式桌面 SSH 会话管理工具**

**🔗 开源地址**: https://github.com/MyNetInfo/LdSSHManager  
**🌐 官网地址（下载）**: https://www.gxlidang.com/soft/ldsshmanager

会话树 · 多标签终端 · 快捷命令 · SFTP · 云端同步 · Vault 加密，专为多主机运维与远程开发打造

[![Platform](https://img.shields.io/badge/平台-Windows%2010%2F11%20x64%20%2B%20Linux%20%2B%20macOS-blue)](#)
[![Version](https://img.shields.io/badge/版本-v0.8.46-green)](#)
[![Wails](https://img.shields.io/badge/Wails-v2-9cf)](#)
[![Vue](https://img.shields.io/badge/Vue-3-42b883)](#)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8)](#)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)

[功能特性](#-核心特性) · [界面预览](#-界面预览) · [安全机制](#-安全有保证为什么可以放心用) · [下载安装](#-下载安装) · [构建开发](#-构建开发) · [问题反馈](#-问题反馈)

</div>

---

## 📖 项目简介

**LdSSHManager** 是一款基于 **Wails v2 + Go + Vue 3** 的跨平台桌面 SSH 会话管理工具（**Windows 10/11、Linux x64、macOS（Apple Silicon）**）。它把多主机运维中最高频的能力——**会话分组与一键连接、xterm 高性能终端、常用命令快捷执行、远端文件浏览编辑、多端配置同步、数据加密保护**——整合进一个轻量、原生体验的桌面应用，让你摆脱在终端 / PuTTY / WinSCP / 记事本之间来回切换的零碎体验。

更重要的是，它从第一天起就坚持三个原则：

- 🧡 **开源**：完整源代码公开在 GitHub，人人可审计、可复现构建，没有黑盒，没有后门；
- 🛠️ **可定制**：技术栈开放（Go + Vue 3 + xterm），你可以按自己的需求改主题、加功能、集成进内部体系；
- 🔐 **安全有保证**：本地数据加密存储、云端同步端到端加密，无论单机使用还是上云同步，你的凭据与数据都只属于你。

- 开源地址：<https://github.com/MyNetInfo/LdSSHManager>
- 官网（下载）：<https://www.gxlidang.com/soft/ldsshmanager>
- 当前版本：**v0.8.46**

---

## ✨ 核心特性

### 会话与连接
- 🌳 **会话树分组管理**：外网 / 内网 / 自定义分组，双击一键连接，支持导入导出。
- ⚡ **两步连接**：双击立即打开标签显示 `connecting`，后台异步建立 SSH，状态实时反馈 `connected` / `failed`，不等待。
- 🔁 **SFTP cd 联动**：终端里执行 `cd`，文件管理器自动跟随切换目录，浏览与操作无缝衔接。
- 🌐 **代理连接**：支持代理连接 SSH，适配受限网络环境。

### 终端体验
- 🖥️ **xterm 终端 + WebGL 渲染**：TUI / top / vim 等高频重绘场景更清晰流畅，真实 PTY 尺寸随窗口自适应。
- 📑 **多标签并行**：多台服务器同时连接、独立操作。
- ⌨️ **快捷命令库**：分类管理常用命令片段，一键执行，可附带回车自动确认。
- 🔄 **断线保护**：连接断开后冻结输入、停止 `send error` 刷屏，明确提示 `[connection closed]`。

### 文件与同步
- 📁 **SFTP 文件管理**：内置文件管理器，浏览 / 上传下载 / 重命名 / 删除。
- ☁️ **云端同步（端到端加密）**：会话 / 快捷命令 / 配置加密上传云端，多端共享、随时迁移，支持版本回溯与冲突处理。
- 💾 **本地加密存储**：全库 SQLCipher（AES-256）加密，密钥由锁屏密码派生，二进制附件存 `data/` 目录。

### 安全与体验
- 🔒 **锁屏保护**：闲置自动锁屏（时长可在系统设置中调整，0=不自动锁）。
- 🗝️ **加密导出 / 导入**：全量数据可加密备份 / 迁移。
- 📊 **服务器状态实时面板**：底部状态栏显示 CPU / 内存 / 磁盘 / 进程 / 网络 / 运行时间等（SecureCRT 同款布局）。
- 🌗 **多主题**：浅色 / 深色自由切换。
- 🌍 **多语言界面**：简中 / 繁中 / English / 한국어 / 日本語 / Русский / Deutsch / Français。
- 🔌 **环境自检 + 一键安装**（开发者）：`INIT.bat` 自动检测并安装 Go / Node / MSYS2-GCC / NSIS / Wails CLI；`MAKE.bat` 普通权限即可构建。

---

## 📸 界面预览

### 🖥️ 主界面 · 会话树 + 终端 + 文件管理器
左侧会话树按"外网 / 内网"分组，双击一键连接；中间多标签 xterm 终端实时执行命令；右侧 SFTP 文件管理器浏览 `/root` 目录；底部状态栏实时显示远程主机的系统 / 磁盘 / CPU / 内存 / 进程 / IO / 连接 / 网卡 / 运行 / 负载（SecureCRT 风格信息栏）。
![Main](images/11.jpg)

### ⌨️ 快捷命令栏
左侧"快捷命令"面板按分类整理常用命令（基础配置 / 密码 / 密钥 / Easytier / MacOS / AI / 系统检查 / 防火墙 / 端口占用…），一键在终端执行。
![QuickCommand](images/22.jpg)

### 📤 数据导出（锁屏密码加密）
"导出数据"明确提示：导出内容**使用锁屏密码加密**，密码丢失即无法恢复，请务必牢记。
![Export](images/33.jpg)

### ☁️ 云端版本选择
云端下载 / 同步时可选择历史版本（`版本 1 ~ 版本 7 (最新)`）并带时间戳，便于回溯与冲突处理。
![CloudVersions](images/44.jpg)

### 🔐 输入云端加密密码
云端同步 / 上传前需输入**加密 / 解密密码**（≥6 位）：数据在本地加密后上传，云端仅存密文；下载时用同一密码解密。
![CloudPassword](images/55.jpg)

### 🌐 多语言切换
顶部 **Language** 下拉支持 7 种语言一键切换（English / 繁體中文 / 한국어 / 日本語 / Русский / Deutsch / Français，标签恒为英文，任何语言下都能认出）。
![Language](images/66.jpg)

### ⚙️ 系统设置
简洁的"系统设置"弹窗：**终端风格**（浅色 / 深色 / 自定义）、**终端底部留空行数**、**自动锁屏**时长（分钟，0=不自动锁）。
![Settings](images/77.jpg)

---

## 🛡️ 安全有保证 · 为什么可以放心用

> **一句话总结：本地数据，锁屏密码加密；云端数据，加密密码端到端加密；全程无明文落盘、无明文传输。**

### 1️⃣ 开源 = 透明可审计
完整源码公开在 <https://github.com/MyNetInfo/LdSSHManager>。你可以：
- **逐行审计**：加密、存储、网络行为全部摊开，不存在"我们也不知道它上传了什么"的黑盒；
- **自行构建**：`INIT.bat` + `MAKE.bat` 一条龙构建，产物可与官方发布逐字节对比；
- **按需定制**：改界面、加功能、对接你的内部系统，二次开发无障碍。

### 2️⃣ 锁屏密码 = 本地数据的唯一钥匙
- 你的锁屏密码用于**派生本地数据库的加密密钥**（PBKDF2 → SQLCipher / AES-256）；
- 数据库**全库加密**：连接信息、账号密码、快捷命令、设置全部密文存储；
- **没有密码就打不开**——即使数据库文件被整个拷走，也只是一堆无法解密的字节；
- 导出 / 导入备份同样由锁屏密码加解密，备份文件安全可靠。

### 3️⃣ 云端同步 = 独立的"加密 / 解密密码"，端到端保护
- 云同步使用**独立于锁屏密码的加密密码**（≥6 位）；
- **上传前本地加密**，云端服务器只收到并保存密文；**下载时本地解密**——云端无法读取你的连接配置；
- 云端数据与本地数据**两把不同的钥匙**，职责隔离，任一处泄露都不影响另一处；
- 忘记加密密码，云端数据同样**无法恢复**——这正是端到端加密该有的样子。

### 4️⃣ 本地用、上云用，安全都不打折
| 使用方式 | 数据去向 | 安全保障 |
|---|---|---|
| **仅本地使用** | 全部数据加密存在本机 | SQLCipher 全库加密 + 锁屏密码派生密钥 + 可选自动锁屏 |
| **同步到云端** | 会话 / 快捷命令 / 配置加密上传 | 端到端加密（上传前加密 / 下载时解密），云端仅见密文 |
| **导出 / 备份** | 本地备份文件 | 锁屏密码加密，可安全拷贝、异地保存 |

---

## 🏗️ 技术栈

| 层 | 选型 | 说明 |
|---|---|---|
| 桌面框架 | **Wails v2** | Go + Web，原生窗口（Windows: WebView2 / Linux: WebKit2GTK / macOS: WKWebView） |
| 后端 | **Go 1.25+** | Wails 绑定、SSH 客户端（CGO 依赖 go-sqlcipher）、并发数据泵 |
| 前端 | **Vue 3 (TypeScript) + Vite** | 组件化、类型安全 |
| 终端 | **xterm.js + WebGL 渲染** | 高性能 TUI 渲染 |
| 数据库 | **SQLite + SQLCipher (AES-256)** | 本地加密数据库，密钥由锁屏密码派生 |
| 通信 | Wails bindings + Go channels | 前后端事件 / 调用 |

---

## 📦 下载安装

### 获取方式

| 渠道 | 地址 | 说明 |
|---|---|---|
| 🌐 **官网** | <https://www.gxlidang.com/soft/ldsshmanager> | 下载便携版 / 安装版 |
| 🐙 **GitHub** | <https://github.com/MyNetInfo/LdSSHManager> | 源码、Issue、Releases |

### 安装包

| 包 | 文件 | 适用场景 |
|---|---|---|
| 🟢 **Windows 绿色便携版** | `LdSSHManager-x64.exe` | 免安装，双击即用，适合临时使用 / U 盘携带 |
| 📦 **Windows NSIS 安装版** | `LdSSHManager-Install-x64.exe` | 标准 Windows 安装包，可自定义安装目录、桌面 / 开始菜单快捷方式、卸载 |
| 🐧 **Linux 版** | `LdSSHManager-linux-x64` | 可执行文件（`chmod +x` 后直接运行），需 GTK3 / WebKit2GTK 桌面依赖 |
| 🍎 **macOS 版（Apple Silicon）** | `LdSSHManager-macos-arm64.dmg` | 磁盘镜像（M 系列芯片；未签名，首次需右键 → 打开） |
| 🍎 **macOS 版（Intel）** | `LdSSHManager-macos-x64.dmg` | 磁盘镜像（Intel Mac；未签名，首次需右键 → 打开） |

> Windows 下载后如提示"未知发布者"，点击"更多信息"→"仍要运行"即可（未购买商业代码签名证书，可从开源地址自行校验 / 构建）。
>
> **自动构建**：推送代码到 GitHub `main` 分支后，GitHub Actions 会自动构建 **Windows（便携版 + 安装版）与 Linux** 产物并发布到仓库 [Releases](https://github.com/MyNetInfo/LdSSHManager/releases) 的 `latest`（每次 push 自动更新）。

### 系统要求

- **操作系统**：
  - **Windows** 10 / 11（x64）
  - **Linux** x64（需 GTK3、WebKit2GTK 4.1 等桌面依赖；主流发行版可 `sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev` 等）
  - **macOS** Apple Silicon（M 系列，macOS 12+）与 Intel（macOS 12+）；未签名，首次运行需右键 → 打开
- **WebView2 Runtime**：仅 Windows 需要——Win11 已预装；Win10 使用**安装版**已内嵌，便携版首次启动按提示安装一次即可
- **屏幕分辨率**：建议 ≥ 1280×720

---

## 🛠️ 构建开发

开发者自行构建的环境要求见 [AGENTS.md](./AGENTS.md)。

### 源码结构

```
LdSSHManager/
├── app.go               # Wails 绑定方法
├── internal/            # Go 后端业务（store / vault / sshclient）
├── frontend/            # Vue3 前端
│   └── src/components/  # 主要 UI 组件
├── MAKE.bat             # 构建脚本（普通权限）
├── INIT.bat             # 环境检测 + 安装脚本（新机手动执行一次，需管理员）
├── DEV.bat              # wails dev 热开发
├── main.go              # Wails 入口
├── bump_version.mjs     # 提交前自动递增版本号
└── ___docs___/CHANGES.md # 重大改动记录
```

### 快速开始

```powershell
# 1. 首次 / 新机：手动执行一次环境安装（双击 INIT.bat 或在管理员 cmd 中执行，弹一次 UAC）
INIT.bat

# 2. 构建（普通权限，无需管理员）
MAKE.bat
```

`MAKE.bat` 自动完成：svn 更新 → 拼 PATH → 环境检查 → `wails build -platform windows/amd64 -nsis -webview2 embed` → 产物移至项目根目录。

---

## 🌐 多语言

- 🇨🇳 简体中文（默认） · 🇹🇼 繁體中文 · 🇺🇸 English · 🇰🇷 한국어 · 🇯🇵 日本語 · 🇷🇺 Русский · 🇩🇪 Deutsch · 🇫🇷 Français

顶部 **Language** 按钮（恒为英文标签）一键切换；语言包集中维护于 `frontend/src/i18n.ts`，欢迎提交新语言翻译。

---

## ❓ 常见问题

<details>
<summary><b>下载后 Windows 提示"未知发布者"怎么办？</b></summary>
正常现象：项目未购买商业代码签名证书。点击"更多信息"→"仍要运行"即可。你也可以从开源地址获取源码自行构建，或使用 GitHub Actions 产物并自行校验哈希。
</details>

<details>
<summary><b>忘了锁屏密码怎么办？数据还能恢复吗？</b></summary>
**不能恢复**——这正是加密的意义所在：数据库密钥由锁屏密码派生，密码丢失 = 数据无法解密。建议：
1. 定期用"导出数据"生成加密备份（同一锁屏密码可导入恢复）；
2. 在密码管理器中妥善保存。
</details>

<details>
<summary><b>云端同步的"加密 / 解密密码"忘了怎么办？</b></summary>
同样**无法恢复**——加密密码独立于锁屏密码，云端只存密文，密码丢了谁都解不开。请务必在密码管理器中记录。
</details>

<details>
<summary><b>Win10 首次启动提示安装 WebView2？</b></summary>
下载 **NSIS 安装版**已内嵌 WebView2，无需额外操作；便携版按提示安装一次即可（约 100MB，一次性）。
</details>

<details>
<summary><b>我想二次开发 / 定制，从哪开始？</b></summary>
见[构建开发](#-构建开发)与源码结构说明。核心 UI 组件在 `frontend/src/components/`，后端在 `internal/`；欢迎 Fork / PR / Issue 交流。
</details>

---

## 🔗 链接

- 🐙 **开源地址**：<https://github.com/MyNetInfo/LdSSHManager>
- 🌐 **官网（下载）**：<https://www.gxlidang.com/soft/ldsshmanager>
- 🐛 **问题反馈**：GitHub Issues / 官网评论区

---

## 🤝 致谢

- 终端渲染：[xterm.js](https://xtermjs.org/) & [xterm-webgl](https://github.com/xtermjs/xterm.js)
- 桌面框架：[Wails](https://wails.io/)
- 加密：[SQLCipher](https://www.zetetic.net/sqlcipher/)

---

## 📄 License

本项目采用 **MIT License** 开源协议，详见 [`LICENSE`](LICENSE) 文件（Copyright © 2026 MyNetInfo）。

> ⚠️ 本项目代码以"现状"提供；开源意味着透明可审计，使用前请自行评估并按当地法律法规合规使用。

---

<sub align="center">开源 · 透明 · 可定制 · Made for SSH lovers 💙</sub>
