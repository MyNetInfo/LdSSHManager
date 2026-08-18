package config

import (
	"os"
	"path/filepath"
)

// baseDir returns the directory of the current executable.
// It is used as the root for all application data.
func baseDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// DataDir returns the directory for binary attachments (images, keys, etc.).
func DataDir() string {
	p := filepath.Join(baseDir(), "data")
	_ = os.MkdirAll(p, 0755)
	return p
}

// DBPath returns the path to the SQLite database file.
func DBPath() string {
	return filepath.Join(baseDir(), "LdSSHManager.db")
}

// WebView2Dir returns the WebView2 user-data directory (sibling of data dir).
// Using a fixed path lets the startup guard identify and clean up our own
// leftover WebView2 processes from a previous abnormal exit.
func WebView2Dir() string {
	p := filepath.Join(baseDir(), "webview2")
	_ = os.MkdirAll(p, 0755)
	return p
}
