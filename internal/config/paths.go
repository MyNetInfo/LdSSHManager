package config

import (
	"os"
	"path/filepath"
)

// baseDir returns the directory of the current executable.
func baseDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// userDataDir returns the per-user data directory (MSIX-safe, always writable).
//   - Windows: %LOCALAPPDATA%\LdSSHManager
//   - Linux:   ~/.config/LdSSHManager
//   - macOS:   ~/Library/Application Support/LdSSHManager
func userDataDir() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		dir = baseDir() // 兜底: 系统不可用时退回 exe 目录
	}
	p := filepath.Join(dir, "LdSSHManager")
	_ = os.MkdirAll(p, 0755)
	return p
}

// isWritable reports whether dir allows creating files.
// Used to detect "portable / green" installs (exe dir writable) vs
// MSIX / Program Files installs (exe dir read-only).
func isWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".ldwrite_test_*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

// dataRoot returns the root directory for all application data.
//   - exe 目录可写 (便携版 / 绿色版): 沿用旧行为, 数据放在 exe 同目录;
//   - exe 目录只读 (MSIX 商店版 / Program Files): 数据放在用户数据目录,
//     避免写入只读安装目录导致数据丢失。
func dataRoot() string {
	exeDir := baseDir()
	if isWritable(exeDir) {
		return exeDir
	}
	return userDataDir()
}

// DataDir returns the directory for binary attachments (images, keys, etc.).
func DataDir() string {
	p := filepath.Join(dataRoot(), "data")
	_ = os.MkdirAll(p, 0755)
	return p
}

// DBPath returns the path to the SQLite database file.
func DBPath() string {
	return filepath.Join(dataRoot(), "LdSSHManager.db")
}

// WebView2Dir returns the WebView2 user-data directory (sibling of data dir).
// Using a fixed path lets the startup guard identify and clean up our own
// leftover WebView2 processes from a previous abnormal exit.
func WebView2Dir() string {
	p := filepath.Join(dataRoot(), "webview2")
	_ = os.MkdirAll(p, 0755)
	return p
}
