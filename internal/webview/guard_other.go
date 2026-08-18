//go:build !windows

package webview

// CleanupOrphanWebView2 非 Windows 平台无 WebView2 进程, 直接返回 0。
func CleanupOrphanWebView2(exeBase string, matchAny ...string) (int, error) {
	return 0, nil
}
