<!-- Language switch -->
[**简体中文**](README.md) | [English](README.en.md)

<div align="center">

# LdSSHManager

**A desktop SSH session manager — session tree, multi-tab terminal, quick commands, SFTP, cloud sync, and Vault encryption, all in one app for multi-host ops and remote development**

[![Platform](https://img.shields.io/badge/Platform-Windows%2010%2F11%20x64-blue)](#)
[![Version](https://img.shields.io/badge/Version-v0.8.46-green)](#)
[![Wails](https://img.shields.io/badge/Wails-v2-9cf)](#)
[![Vue](https://img.shields.io/badge/Vue-3-42b883)](#)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8)](#)
[![License](https://img.shields.io/badge/License-TBD-lightgrey)](LICENSE)

[Website](https://www.gxlidang.com/soft/ldsshmanager) · [Download](#-download--install) · [Build](#-build--development) · [Feedback](#-feedback)

</div>

---

## 📖 Overview

**LdSSHManager** is a desktop SSH session management tool built with **Wails v2 + Go + Vue 3**, designed for multi-host operations, remote development, and server debugging. It consolidates the most common SSH workflows — **session grouping & connection, xterm terminal, quick commands, remote file browsing/editing, multi-device config sync, and data encryption** — into a single lightweight, native-feeling Windows application, so you no longer have to switch between terminals, PuTTY, WinSCP, and notepads.

The app follows a **local-first and encryption-first** philosophy: all sensitive data (AES-256), images/attachments, and connection credentials are encrypted and stored locally. Cloud sync is optional and end-to-end encrypted. It also bundles a quick-command library, a real-time server status panel, multiple languages and themes — ideal for individual developers and ops teams alike.

- Website: <https://www.gxlidang.com/soft/ldsshmanager>
- Current version: **v0.8.46**

---

## ✨ Features

### Sessions & Connections
- 🌳 **Session tree with grouping**: External / Internal / custom groups; double-click to connect; import & export.
- ⚡ **Two-step connect**: Double-click opens a tab in `connecting` state immediately, SSH handshake runs in the background, status flips to `connected` / `failed` — no waiting.
- 🔁 **SFTP cd sync**: Running `cd` in the terminal auto-updates the file manager's current directory.
- 🌐 **Proxy support**: Connect to SSH through proxies for restricted networks.

### Terminal Experience
- 🖥️ **xterm + WebGL renderer**: Crisper, smoother TUI redraw (top / htop / vim); real PTY size adapts to window resizing.
- 📑 **Multi-tab parallel**: Multiple servers connected and managed at the same time.
- ⌨️ **Quick command library**: Categorized command snippets, one-click execution, optional auto-Enter.
- 🔄 **Disconnect protection**: After a connection drops, input is frozen to stop `send error` spam and a `[connection closed]` notice is shown.

### Files & Sync
- 📁 **SFTP file manager**: Browse, upload/download, rename, delete.
- ☁️ **Cloud sync**: Sessions / quick commands / configs can be encrypted-uploaded to the cloud, shared across devices, migrated anytime. Version history and conflict handling supported.
- 💾 **Encrypted local storage**: All data SQLCipher (AES-256) encrypted; the key is derived from your lock-screen password; binary attachments live in `data/`.

### Security & UX
- 🔒 **Lock screen**: Auto-lock on idle (configurable in System Settings, 0=disabled) to prevent shoulder-surfing.
- 🗝️ **Export / Import**: Full encrypted export & import for backup and migration.
- 📊 **Real-time server status panel**: Bottom bar shows CPU / memory / disk / process / network / uptime etc. (SecureCRT-style).
- 🌗 **Multiple themes**: Light / dark, free to switch.
- 🌍 **Multi-language UI**: Simplified Chinese / Traditional Chinese / English / Korean / Japanese / Russian / German / French.
- 🔌 **Self-installing build env (dev)**: `INIT.bat` auto-detects/installs Go, Node, MSYS2-GCC, NSIS, Wails CLI; `MAKE.bat` builds with normal user rights (no admin required).

---

## 📸 Screenshots

### 🖥️ Main UI — Session tree + Terminal + File Manager
Session tree grouped by "External / Internal" on the left, one-click connect; multi-tab xterm terminal in the middle running real commands; SFTP file manager browsing `/root` on the right; bottom status bar shows remote system/disk/CPU/memory/process/IO/connection/network/uptime/load in real time (SecureCRT-style info bar).
![Main](images/11.jpg)

### ⌨️ Quick Command Panel
Left "Quick Command" panel lists commands by category (`基础配置 / 密码 / 密钥 / Easytier / MacOS / AI / 常用命令 / 系统检查 / 防火墙 / 端口占用 / ...`); double-click or use the hotkey to run in the terminal.
![QuickCommand](images/22.jpg)

### 📤 Export Data (lock-screen-password encrypted)
"Export Data" dialog reminds you: **the export is encrypted with your lock-screen password — if you forget it, recovery is impossible. Remember it well.**
![Export](images/33.jpg)

### ☁️ Cloud Version Picker
When downloading / syncing from the cloud, you can pick a historical version (`版本 1 ~ 版本 7 (最新)`) with timestamps for rollback and conflict resolution.
![CloudVersions](images/44.jpg)

### 🔐 Cloud Encryption Password
Cloud sync / upload requires an encryption password (≥ 6 characters) for end-to-end protection of your connection config.
![CloudPassword](images/55.jpg)

### 🌐 Language Switcher
Top-bar **Language** dropdown supports 7 languages in one click: English / 繁體中文 / 한국어 / 日本語 / Русский / Deutsch / Français (the label is always shown in English for easy recognition).
![Language](images/66.jpg)

### ⚙️ System Settings
Clean "System Settings" dialog: **Terminal Style** (light / dark / custom), **Terminal bottom blank rows**, **Auto Lock** minutes (0 = disabled).
![Settings](images/77.jpg)

---

## 🏗️ Tech Stack

| Layer | Choice | Notes |
|---|---|---|
| Desktop framework | **Wails v2** | Go + Web, native window + WebView2 |
| Backend | **Go 1.25+** | Wails bindings, SSH client (CGO with go-sqlcipher), SQLCipher, concurrent pumps |
| Frontend | **Vue 3 (TypeScript) + Vite** | Componentized, type-safe |
| Terminal | **xterm.js + WebGL renderer** | High-performance TUI rendering |
| Database | **SQLite + SQLCipher (AES-256)** | Local encrypted database; key derived from lock-screen password |
| IPC | Wails bindings + Go channels | Front-end ↔ back-end events and calls |

---

## 🔒 Security & Privacy

| Item | Mechanism |
|---|---|
| Local database | **SQLCipher (AES-256)** full-database encryption; key derived from your lock-screen password; unrecoverable without the correct password |
| Binary attachments | Stored in `data/` next to the exe; not uploaded unless you explicitly use Cloud Sync |
| Lock screen | App auto-locks on idle (configurable duration in System Settings, 0=disabled) |
| Cloud sync | End-to-end encrypted: the user-chosen "encryption password" (≥6 chars) is independent of the lock-screen password; losing it makes the cloud data unrecoverable |
| Data export | Same cloud-encryption rules: encrypted with the lock-screen password |
| In-memory defense | `store` / `vault` paths explicitly guard against `nil` DB / missing rekey to avoid silent data loss |

---

## 📦 Download & Install

### Official
👉 **<https://www.gxlidang.com/soft/ldsshmanager>**

Two packages are provided:

| Package | File | Use case |
|---|---|---|
| 🟢 **Portable (single-file)** | `LdSSHManager-x64.exe` | No installation, double-click to run — great for ad-hoc use or USB drives |
| 📦 **NSIS Installer** | `LdSSHManager-Install-x64.exe` | Standard Windows installer: custom install dir, desktop/Start-menu shortcuts, uninstaller |

> If Windows warns "Unknown publisher" after download, click **More info** → **Run anyway** (no commercial code-signing certificate is in use).

### System requirements

- **OS**: Windows 10 / 11 (x64)
- **WebView2 Runtime**: pre-installed on Win11; on Win10 the **NSIS installer** embeds it so no extra download is needed (the portable build will prompt to install it the first time)
- **Screen resolution**: ≥ 1280×720 recommended

---

## 🛠️ Build & Development

For developers wanting to build from source, see [AGENTS.md](./AGENTS.md).

### Source layout

```
LdSSHManager/
├── app.go               # Wails bindings
├── internal/            # Go backend (store / vault / sshclient)
├── frontend/            # Vue3 frontend
│   └── src/components/  # Main UI components
├── MAKE.bat             # Build script (normal user rights)
├── INIT.bat             # Environment detect + install (run once per machine, needs admin)
├── DEV.bat              # wails dev hot reload
├── main.go              # Wails entry
├── bump_version.mjs     # Auto-bump version before each SVN commit
└── ___docs___/CHANGES.md # Major-change log
```

### Quick start

```powershell
# 1. First time / new machine: install build environment manually
#    (double-click INIT.bat or run it in an admin cmd — one UAC prompt)
INIT.bat

# 2. Build (normal user, no admin required)
MAKE.bat
```

`MAKE.bat` will automatically: update SVN → compose PATH → check environment → run `wails build -platform windows/amd64 -nsis -webview2 embed` → move the artifacts to the project root.

### Before each commit

- Run `node bump_version.mjs` to auto-bump the version in `app.go`'s `GetVersion()`.
- Commit to SVN with the message format `【{hostname}】LdSSHManager - {change description}` and always include the new version.

---

## 🌐 Internationalization

Built-in UI languages:

- 🇨🇳 Simplified Chinese (default)
- 🇹🇼 Traditional Chinese
- 🇺🇸 English
- 🇰🇷 Korean
- 🇯🇵 Japanese
- 🇷🇺 Russian
- 🇩🇪 German
- 🇫🇷 French

The top-bar **Language** button is always labeled in English (so you can find it in any locale) and switches instantly; translation files live in `frontend/src/i18n.ts`.

---

## 📁 Project Structure (key parts)

```
frontend/src/components/
├── SessionTree.vue     # Session tree + new/edit dialog
├── QuickCommandBar.vue # Bottom quick command bar (920px edit dialog)
├── TerminalPanel.vue   # xterm + WebGL + PTY sync
├── TerminalTabs.vue    # Tab bar
├── Topbar.vue          # Top menu + system settings + cloud sync
├── LockScreen.vue      # Lock screen
├── SessionStatsBar.vue # Real-time server status panel
├── FileManager.vue     # SFTP file manager
├── Dialog.vue + dialog.ts # Unified Toast/Confirm/Prompt
└── ...
```

---

## ❓ FAQ

<details>
<summary><b>Windows warns "Unknown publisher" after download — what do I do?</b></summary>
Expected: the project does not use a commercial code-signing certificate. Click **More info** → **Run anyway**.
</details>

<details>
<summary><b>Win10 prompts to install WebView2 on first launch?</b></summary>
The **NSIS installer** (`LdSSHManager-Install-x64.exe`) bundles WebView2, so nothing extra is needed. The portable build will prompt to install it once (~100 MB, one-time).
</details>

<details>
<summary><b>I forgot my lock-screen password — can I recover my data?</b></summary>
**No, it is unrecoverable** — the SQLCipher key is derived from your password. Please:
1. Regularly use **Export Data** to make an encrypted backup (can be re-imported with the same lock-screen password);
2. Store a copy in a password manager.
</details>

<details>
<summary><b>I forgot my cloud encryption password — can I recover the cloud data?</b></summary>
Same answer: **unrecoverable**. The cloud encryption password is independent of the lock-screen password. Store it in a password manager.
</details>

---

## 🔗 Links

- 🌐 **Website (download)**: <https://www.gxlidang.com/soft/ldsshmanager>
- 🐛 **Issues / feedback**: open a GitHub Issue
- 💬 **Suggestions**: website comments / email

---

## 🤝 Acknowledgments

- Terminal: [xterm.js](https://xtermjs.org/) & [xterm-webgl](https://github.com/xtermjs/xterm.js)
- Desktop framework: [Wails](https://wails.io/)
- Encryption: [SQLCipher](https://www.zetetic.net/sqlcipher/)

---

## 📄 License

The open-source license for this repository is described in the [`LICENSE`](LICENSE) file.

> ⚠️ This project is provided "as is". Please evaluate security risks and follow your local laws and regulations before use.

---

<sub align="center">Built with ❤️ · Made for SSH lovers</sub>
